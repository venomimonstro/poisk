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
