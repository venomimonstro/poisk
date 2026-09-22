//go:build integration

package admin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestReviewModerationPreviewIsSessionBoundAndRecomputesRating(t *testing.T){
	pool:=adminIntegrationDB(t);repo,adminUser,session,_:=createIntegrationAdmin(t,pool,"review-moderation");ctx:=context.Background();service:=Service{Store:repo}
	var consumerID int64;email:=fmt.Sprintf("review-mod-%d@example.test",time.Now().UnixNano())
	if err:=pool.QueryRow(ctx,`INSERT INTO consumer_users(email,password_hash) VALUES($1,$2) RETURNING user_id`,email,"integration-password-hash-000000").Scan(&consumerID);err!=nil{t.Fatal(err)}
	var reporterID int64;reporterEmail:=fmt.Sprintf("review-report-%d@example.test",time.Now().UnixNano())
	if err:=pool.QueryRow(ctx,`INSERT INTO consumer_users(email,password_hash) VALUES($1,$2) RETURNING user_id`,reporterEmail,"integration-password-hash-000000").Scan(&reporterID);err!=nil{t.Fatal(err)}
	var placeID int64;name:=fmt.Sprintf("Review moderation place %d",time.Now().UnixNano())
	if err:=pool.QueryRow(ctx,`INSERT INTO organizations(name,normalized_name,status,quality_score,source_count) VALUES($1,lower($1),'ACTIVE',80,1) RETURNING place_id`,name).Scan(&placeID);err!=nil{t.Fatal(err)}
	var reviewID int64
	if err:=pool.QueryRow(ctx,`INSERT INTO organization_reviews(place_id,consumer_user_id,rating,body,status,change_actor_type,change_actor_id,change_reason) VALUES($1,$2,5,'Отзыв для проверки административной модерации.','VISIBLE','USER',$2,'USER_CREATE') RETURNING review_id`,placeID,consumerID).Scan(&reviewID);err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`INSERT INTO organization_review_reports(review_id,reporter_user_id,reason,details) VALUES($1,$2,'SPAM','integration report')`,reviewID,reporterID);err!=nil{t.Fatal(err)}
	var count int64;var avg float64
	if err:=pool.QueryRow(ctx,`SELECT review_count,average_rating::float8 FROM organization_review_stats WHERE place_id=$1`,placeID).Scan(&count,&avg);err!=nil{t.Fatal(err)}
	if count!=1||avg!=5{t.Fatalf("initial stats count=%d avg=%f",count,avg)}

	preview,err:=service.PreviewReviewModeration(ctx,session,reviewID,"HIDE","confirmed abuse report");if err!=nil{t.Fatal(err)}
	otherHash:=bytes.Repeat([]byte{55},32);otherCSRF:=bytes.Repeat([]byte{54},32);other,err:=repo.CreateSession(ctx,adminUser.ID,otherHash,otherCSRF,nil,time.Hour);if err!=nil{t.Fatal(err)};other.Role="OPERATOR"
	if _,err:=service.ApplyReviewModeration(ctx,other,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("foreign session used moderation preview: %v",err)}
	result,err:=service.ApplyReviewModeration(ctx,session,preview.Token);if err!=nil{t.Fatal(err)}
	if result.Status!="HIDDEN"||result.Version!=2{t.Fatalf("result=%+v",result)}
	if _,err:=service.ApplyReviewModeration(ctx,session,preview.Token);!errors.Is(err,ErrPreviewInvalid){t.Fatalf("moderation preview reused: %v",err)}

	if err:=pool.QueryRow(ctx,`SELECT review_count,average_rating::float8 FROM organization_review_stats WHERE place_id=$1`,placeID).Scan(&count,&avg);err!=nil{t.Fatal(err)}
	if count!=0||avg!=0{t.Fatalf("hidden review still affects rating count=%d avg=%f",count,avg)}
	var reportStatus string;var revisions,events,audits int
	if err:=pool.QueryRow(ctx,`SELECT status FROM organization_review_reports WHERE review_id=$1 AND reporter_user_id=$2`,reviewID,reporterID).Scan(&reportStatus);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organization_review_revisions WHERE review_id=$1`,reviewID).Scan(&revisions);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organization_review_moderation_events WHERE review_id=$1 AND action='HIDE'`,reviewID).Scan(&events);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM audit_log WHERE actor_type='ADMIN' AND action='REVIEW_MODERATION_APPLY' AND entity_type='ORGANIZATION_REVIEW' AND entity_id=$1::bigint::text`,reviewID).Scan(&audits);err!=nil{t.Fatal(err)}
	if reportStatus!="RESOLVED"||revisions!=2||events!=1||audits!=1{t.Fatalf("report=%s revisions=%d events=%d audits=%d",reportStatus,revisions,events,audits)}
}
