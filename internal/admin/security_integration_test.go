//go:build integration

package admin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func adminIntegrationDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required for integration tests")}
	pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err:=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)};t.Cleanup(pool.Close);return pool
}

func createIntegrationAdmin(t *testing.T,pool *pgxpool.Pool,suffix string)(*Repository,Admin,Session,[]byte){
	t.Helper();ctx:=context.Background();repo:=NewRepository(pool)
	admin,err:=repo.CreateAdmin(ctx,fmt.Sprintf("hardening-%s-%d@example.test",suffix,time.Now().UnixNano()),"test-hash","OPERATOR");if err!=nil{t.Fatal(err)}
	tokenHash:=bytes.Repeat([]byte{byte(len(suffix)+11)},32);csrfHash:=bytes.Repeat([]byte{byte(len(suffix)+21)},32);ua:=bytes.Repeat([]byte{1},32)
	session,err:=repo.CreateSession(ctx,admin.ID,tokenHash,csrfHash,ua,time.Hour);if err!=nil{t.Fatal(err)};session.Role="OPERATOR";session.Email=admin.Email
	t.Cleanup(func(){_,_=pool.Exec(context.Background(),`DELETE FROM admin_users WHERE admin_id=$1`,admin.ID)})
	return repo,admin,session,tokenHash
}

func TestSessionRevocationInvalidatesToken(t *testing.T){
	pool:=adminIntegrationDB(t);repo,_,session,tokenHash:=createIntegrationAdmin(t,pool,"revoke");ctx:=context.Background()
	if _,err:=repo.SessionByToken(ctx,tokenHash);err!=nil{t.Fatalf("session should exist: %v",err)}
	if err:=repo.RevokeSession(ctx,session.ID,session.AdminID);err!=nil{t.Fatal(err)}
	if _,err:=repo.SessionByToken(ctx,tokenHash);!errors.Is(err,ErrSessionNotFound){t.Fatalf("expected revoked session to fail, got %v",err)}
}

func TestDomainPreviewIsSingleUseAndSessionBound(t *testing.T){
	pool:=adminIntegrationDB(t);repo,admin,session,_:=createIntegrationAdmin(t,pool,"preview");ctx:=context.Background();service:=Service{Store:repo}
	host:=fmt.Sprintf("preview-%d.example.test",time.Now().UnixNano());var domainID int64
	if err:=pool.QueryRow(ctx,`INSERT INTO domains(host) VALUES($1) RETURNING domain_id`,host).Scan(&domainID);err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=pool.Exec(context.Background(),`DELETE FROM domains WHERE domain_id=$1`,domainID)})
	preview,err:=service.PreviewDomainMutation(ctx,session,domainID,"PAUSED","LIMITED");if err!=nil{t.Fatal(err)}
	otherHash:=bytes.Repeat([]byte{99},32);otherCSRF:=bytes.Repeat([]byte{98},32);other,err:=repo.CreateSession(ctx,admin.ID,otherHash,otherCSRF,nil,time.Hour);if err!=nil{t.Fatal(err)};other.Role="OPERATOR"
	if _,err:=service.ApplyDomainMutation(ctx,other,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("foreign session used preview: %v",err)}
	result,err:=service.ApplyDomainMutation(ctx,session,preview.Token);if err!=nil{t.Fatal(err)}
	if result.Status!="PAUSED"||result.Policy!="LIMITED"{t.Fatalf("result=%+v",result)}
	if _,err:=service.ApplyDomainMutation(ctx,session,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("preview reused: %v",err)}
	var status,policy string;if err:=pool.QueryRow(ctx,`SELECT status,policy FROM domains WHERE domain_id=$1`,domainID).Scan(&status,&policy);err!=nil{t.Fatal(err)}
	if status!="PAUSED"||policy!="LIMITED"{t.Fatalf("domain status=%s policy=%s",status,policy)}
}

func TestQueryGapPreviewIsSingleUseAndAudited(t *testing.T){
	pool:=adminIntegrationDB(t);repo,_,session,_:=createIntegrationAdmin(t,pool,"gap");ctx:=context.Background();service:=Service{Store:repo}
	queryHash:=bytes.Repeat([]byte{byte(time.Now().UnixNano()%200+20)},32)
	var gapID int64
	if err:=pool.QueryRow(ctx,`INSERT INTO query_gaps(query_hash,representative_query,state,demand_score,coverage_score,quality_score,freshness_score,spam_score,gap_score,independent_buckets) VALUES($1,'integration query','OPEN',80,20,30,40,5,60,5) RETURNING gap_id`,queryHash).Scan(&gapID);err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=pool.Exec(context.Background(),`DELETE FROM query_gaps WHERE gap_id=$1`,gapID)})
	preview,err:=service.PreviewQueryGapMutation(ctx,session,gapID,"SUPPRESS","integration moderation");if err!=nil{t.Fatal(err)}
	if preview.Before.State!="OPEN"||preview.After.State!="SUPPRESSED"{t.Fatalf("preview=%+v",preview)}
	result,err:=service.ApplyQueryGapMutation(ctx,session,preview.Token);if err!=nil{t.Fatal(err)}
	if result.State!="SUPPRESSED"{t.Fatalf("result=%+v",result)}
	if _,err:=service.ApplyQueryGapMutation(ctx,session,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("preview reused: %v",err)}
	var state string;var auditCount,eventCount int
	if err:=pool.QueryRow(ctx,`SELECT state FROM query_gaps WHERE gap_id=$1`,gapID).Scan(&state);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM audit_log WHERE actor_type='ADMIN' AND action='QUERY_GAP_STATE_APPLY' AND entity_type='QUERY_GAP' AND entity_id=$1::bigint::text`,gapID).Scan(&auditCount);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM query_gap_feedback_events WHERE gap_id=$1 AND action='SUPPRESS'`,gapID).Scan(&eventCount);err!=nil{t.Fatal(err)}
	if state!="SUPPRESSED"||auditCount!=1||eventCount!=1{t.Fatalf("state=%s audit=%d events=%d",state,auditCount,eventCount)}
}

func TestOrganizationReviewPreviewApplyIsSingleUseAndAudited(t *testing.T){
	pool:=adminIntegrationDB(t);repo,adminUser,session,_:=createIntegrationAdmin(t,pool,"org-review");ctx:=context.Background();service:=Service{Store:repo}
	source:=fmt.Sprintf("admin-test-%d",time.Now().UnixNano());if _,err:=pool.Exec(ctx,`INSERT INTO organization_sources(source_key,display_name) VALUES($1,'Admin integration')`,source);err!=nil{t.Fatal(err)}
	var batchID int64;if err:=pool.QueryRow(ctx,`INSERT INTO organization_import_batches(source_key,external_batch_key,mode,status) VALUES($1,$2,'DRY_RUN','PLANNED') RETURNING batch_id`,source,"batch-1").Scan(&batchID);err!=nil{t.Fatal(err)}
	var stagingID int64;if err:=pool.QueryRow(ctx,`INSERT INTO organization_staging_rows(batch_id,source_key,source_record_id,source_row_number,raw_payload,raw_bytes,normalized_name,normalized_address,state,payload_hash) VALUES($1,$2,'record-1',1,'{}',2,'incoming org','test street','PLANNED',$3) RETURNING staging_id`,batchID,source,fmt.Sprintf("%064x",time.Now().UnixNano())).Scan(&stagingID);err!=nil{t.Fatal(err)}
	var placeID int64;if err:=pool.QueryRow(ctx,`INSERT INTO organizations(name,normalized_name,address,normalized_address,quality_score,source_count) VALUES('Candidate Org','candidate org','Test street','test street',80,1) RETURNING place_id`).Scan(&placeID);err!=nil{t.Fatal(err)}
	planHash:=fmt.Sprintf("%064x",time.Now().UnixNano()+1);if _,err:=pool.Exec(ctx,`INSERT INTO organization_import_plans(batch_id,staging_id,action,target_place_id,match_rule,confidence,reason_code,plan_hash) VALUES($1,$2,'REVIEW',NULL,'AMBIGUOUS',70,'AMBIGUOUS_MATCH',$3)`,batchID,stagingID,planHash);err!=nil{t.Fatal(err)}
	var reviewID int64;if err:=pool.QueryRow(ctx,`INSERT INTO organization_merge_review(batch_id,staging_id,candidate_place_id,reason_code,score) VALUES($1,$2,$3,'AMBIGUOUS_MATCH',70) RETURNING review_id`,batchID,stagingID,placeID).Scan(&reviewID);err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=pool.Exec(context.Background(),`DELETE FROM organization_sources WHERE source_key=$1`,source);_,_=pool.Exec(context.Background(),`DELETE FROM organizations WHERE place_id=$1`,placeID)})
	preview,err:=service.PreviewOrganizationReview(ctx,session,reviewID,"MERGE","verified candidate");if err!=nil{t.Fatal(err)}
	otherHash:=bytes.Repeat([]byte{77},32);otherCSRF:=bytes.Repeat([]byte{76},32);other,err:=repo.CreateSession(ctx,adminUser.ID,otherHash,otherCSRF,nil,time.Hour);if err!=nil{t.Fatal(err)};other.Role="OPERATOR"
	if _,err:=service.ApplyOrganizationReview(ctx,other,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("foreign session used org preview: %v",err)}
	result,err:=service.ApplyOrganizationReview(ctx,session,preview.Token);if err!=nil{t.Fatal(err)};if result.Status!="MERGED"{t.Fatalf("result=%+v",result)}
	if _,err:=service.ApplyOrganizationReview(ctx,session,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("org preview reused: %v",err)}
	var action string;var target *int64;var eventCount,auditCount int
	if err:=pool.QueryRow(ctx,`SELECT action,target_place_id FROM organization_import_plans WHERE staging_id=$1`,stagingID).Scan(&action,&target);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organization_import_events WHERE batch_id=$1 AND staging_id=$2 AND action='REVIEW_DECISION'`,batchID,stagingID).Scan(&eventCount);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM audit_log WHERE actor_type='ADMIN' AND action='ORG_REVIEW_APPLY' AND entity_type='ORG_REVIEW' AND entity_id=$1::bigint::text`,reviewID).Scan(&auditCount);err!=nil{t.Fatal(err)}
	if action!="UPDATE"||target==nil||*target!=placeID||eventCount!=1||auditCount!=1{t.Fatalf("action=%s target=%v events=%d audit=%d",action,target,eventCount,auditCount)}
}
