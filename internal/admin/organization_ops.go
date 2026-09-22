package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type OrganizationReviewPreview struct {
	Token string `json:"preview_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Review OrganizationReviewRow `json:"review"`
	Decision string `json:"decision"`
	Reason string `json:"reason"`
}

func validReviewDecision(v string)bool{switch v{case "MERGE","CREATE_NEW","REJECT":return true};return false}

func (s Service) PreviewOrganizationReview(ctx context.Context,session Session,reviewID int64,decision,reason string)(OrganizationReviewPreview,error){
	if s.Store==nil||s.Store.db==nil||reviewID<=0{return OrganizationReviewPreview{},ErrInvalidCredential};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return OrganizationReviewPreview{},err}
	decision=strings.ToUpper(strings.TrimSpace(decision));reason=strings.TrimSpace(reason);if !validReviewDecision(decision)||len([]rune(reason))<2||len([]rune(reason))>160{return OrganizationReviewPreview{},ErrInvalidCredential}
	var row OrganizationReviewRow
	err:=s.Store.db.QueryRow(ctx,`SELECT r.review_id,r.batch_id,r.staging_id,r.candidate_place_id,o.name,COALESCE(st.normalized_name,''),COALESCE(st.normalized_address,''),COALESCE(o.normalized_address,''),r.reason_code,r.score,r.status,r.created_at
FROM organization_merge_review r JOIN organization_staging_rows st ON st.staging_id=r.staging_id JOIN organizations o ON o.place_id=r.candidate_place_id WHERE r.review_id=$1`,reviewID).Scan(&row.ReviewID,&row.BatchID,&row.StagingID,&row.CandidatePlaceID,&row.CandidateName,&row.IncomingName,&row.IncomingAddress,&row.CandidateAddress,&row.ReasonCode,&row.Score,&row.Status,&row.CreatedAt)
	if errors.Is(err,pgx.ErrNoRows){return OrganizationReviewPreview{},ErrNotFound};if err!=nil{return OrganizationReviewPreview{},err};if row.Status!="OPEN"{return OrganizationReviewPreview{},ErrPreviewInvalid}
	token,hash,err:=RandomToken(24);if err!=nil{return OrganizationReviewPreview{},err};expires:=time.Now().UTC().Add(5*time.Minute)
	payload,_:=json.Marshal(map[string]any{"review_id":row.ReviewID,"batch_id":row.BatchID,"staging_id":row.StagingID,"candidate_place_id":row.CandidatePlaceID,"decision":decision,"reason":reason,"score":row.Score})
	if _,err=s.Store.db.Exec(ctx,`INSERT INTO admin_action_previews(admin_id,session_id,token_hash,action_type,target_type,target_id,payload,expires_at) VALUES($1,$2,$3,'ORG_REVIEW_DECISION','ORG_REVIEW',$4,$5::jsonb,$6)`,session.AdminID,session.ID,hash,reviewID,string(payload),expires);err!=nil{return OrganizationReviewPreview{},err}
	_ = s.Store.SecurityEvent(ctx,&session.AdminID,"ORG_REVIEW_PREVIEW",true,string(payload));return OrganizationReviewPreview{Token:token,ExpiresAt:expires,Review:row,Decision:decision,Reason:reason},nil
}

func (s Service) ApplyOrganizationReview(ctx context.Context,session Session,token string)(OrganizationReviewRow,error){
	if s.Store==nil||s.Store.db==nil||strings.TrimSpace(token)==""{return OrganizationReviewRow{},ErrPreviewInvalid};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return OrganizationReviewRow{},err}
	tx,err:=s.Store.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return OrganizationReviewRow{},err};defer func(){_=tx.Rollback(ctx)}();hash:=HashToken(token)
	var previewID,reviewID int64;var raw []byte
	err=tx.QueryRow(ctx,`SELECT preview_id,target_id,payload FROM admin_action_previews WHERE admin_id=$1 AND session_id=$2 AND token_hash=$3 AND action_type='ORG_REVIEW_DECISION' AND target_type='ORG_REVIEW' AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`,session.AdminID,session.ID,hash).Scan(&previewID,&reviewID,&raw);if errors.Is(err,pgx.ErrNoRows){return OrganizationReviewRow{},ErrPreviewInvalid};if err!=nil{return OrganizationReviewRow{},err}
	var p struct{ReviewID int64 `json:"review_id"`;BatchID int64 `json:"batch_id"`;StagingID int64 `json:"staging_id"`;CandidatePlaceID int64 `json:"candidate_place_id"`;Decision string `json:"decision"`;Reason string `json:"reason"`};if err:=json.Unmarshal(raw,&p);err!=nil||p.ReviewID!=reviewID||!validReviewDecision(p.Decision){return OrganizationReviewRow{},ErrPreviewInvalid}
	var currentStatus string;var currentCandidate,currentStaging,currentBatch int64
	if err:=tx.QueryRow(ctx,`SELECT status,candidate_place_id,staging_id,batch_id FROM organization_merge_review WHERE review_id=$1 FOR UPDATE`,reviewID).Scan(&currentStatus,&currentCandidate,&currentStaging,&currentBatch);errors.Is(err,pgx.ErrNoRows){return OrganizationReviewRow{},ErrNotFound}else if err!=nil{return OrganizationReviewRow{},err};if currentStatus!="OPEN"||currentCandidate!=p.CandidatePlaceID||currentStaging!=p.StagingID||currentBatch!=p.BatchID{return OrganizationReviewRow{},ErrPreviewInvalid}
	var planID int64;if err:=tx.QueryRow(ctx,`SELECT p.plan_id FROM organization_import_plans p JOIN organization_import_batches b ON b.batch_id=p.batch_id WHERE p.staging_id=$1 AND p.action='REVIEW' AND b.status='PLANNED' FOR UPDATE OF p,b`,p.StagingID).Scan(&planID);err!=nil{return OrganizationReviewRow{},ErrPreviewInvalid}
	action:="CREATE";var target any=nil;reasonCode:="";if p.Decision=="MERGE"{action="UPDATE";target=p.CandidatePlaceID};if p.Decision=="REJECT"{action="REJECT";reasonCode="MANUAL_REJECT"}
	h:=sha256.Sum256([]byte(fmt.Sprintf("admin-review:%d:%d:%s:%v",planID,p.StagingID,action,target)));planHash:=hex.EncodeToString(h[:])
	if _,err=tx.Exec(ctx,`UPDATE organization_import_plans SET action=$2,target_place_id=$3,match_rule='MANUAL_REVIEW',confidence=100,reason_code=NULLIF($4,''),plan_hash=$5 WHERE plan_id=$1`,planID,action,target,reasonCode,planHash);err!=nil{return OrganizationReviewRow{},err}
	switch p.Decision{case "MERGE":_,err=tx.Exec(ctx,`UPDATE organization_merge_review SET status=CASE WHEN candidate_place_id=$2 THEN 'MERGED' ELSE 'REJECTED' END,decided_by=$3,decided_at=now() WHERE staging_id=$1 AND status='OPEN'`,p.StagingID,p.CandidatePlaceID,fmt.Sprintf("admin:%d",session.AdminID));case "CREATE_NEW":_,err=tx.Exec(ctx,`UPDATE organization_merge_review SET status='CREATE_NEW',decided_by=$2,decided_at=now() WHERE staging_id=$1 AND status='OPEN'`,p.StagingID,fmt.Sprintf("admin:%d",session.AdminID));case "REJECT":_,err=tx.Exec(ctx,`UPDATE organization_merge_review SET status='REJECTED',decided_by=$2,decided_at=now() WHERE staging_id=$1 AND status='OPEN'`,p.StagingID,fmt.Sprintf("admin:%d",session.AdminID));if err==nil{_,err=tx.Exec(ctx,`UPDATE organization_staging_rows SET state='REJECTED',rejection_code='MANUAL_REJECT',rejection_detail=$2,updated_at=now() WHERE staging_id=$1`,p.StagingID,p.Reason)}};if err!=nil{return OrganizationReviewRow{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO organization_import_events(batch_id,staging_id,action,details) VALUES($1,$2,'REVIEW_DECISION',jsonb_build_object('decision',$3,'actor',$4,'candidate_place_id',$5,'reason',$6))`,p.BatchID,p.StagingID,p.Decision,fmt.Sprintf("admin:%d",session.AdminID),p.CandidatePlaceID,p.Reason);err!=nil{return OrganizationReviewRow{},err}
	if _,err=tx.Exec(ctx,`UPDATE admin_action_previews SET consumed_at=now() WHERE preview_id=$1`,previewID);err!=nil{return OrganizationReviewRow{},err}
	details,_:=json.Marshal(map[string]any{"review_id":reviewID,"staging_id":p.StagingID,"batch_id":p.BatchID,"decision":p.Decision,"reason":p.Reason,"preview_id":previewID});if _,err=tx.Exec(ctx,`INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details) VALUES('ADMIN',$1::bigint::text,'ORG_REVIEW_APPLY','ORG_REVIEW',$2::bigint::text,$3::jsonb)`,session.AdminID,reviewID,string(details));err!=nil{return OrganizationReviewRow{},err}
	var out OrganizationReviewRow;if err=tx.QueryRow(ctx,`SELECT r.review_id,r.batch_id,r.staging_id,r.candidate_place_id,o.name,COALESCE(st.normalized_name,''),COALESCE(st.normalized_address,''),COALESCE(o.normalized_address,''),r.reason_code,r.score,r.status,r.created_at FROM organization_merge_review r JOIN organization_staging_rows st ON st.staging_id=r.staging_id JOIN organizations o ON o.place_id=r.candidate_place_id WHERE r.review_id=$1`,reviewID).Scan(&out.ReviewID,&out.BatchID,&out.StagingID,&out.CandidatePlaceID,&out.CandidateName,&out.IncomingName,&out.IncomingAddress,&out.CandidateAddress,&out.ReasonCode,&out.Score,&out.Status,&out.CreatedAt);err!=nil{return OrganizationReviewRow{},err};if err=tx.Commit(ctx);err!=nil{return OrganizationReviewRow{},err};_ = s.Store.SecurityEvent(ctx,&session.AdminID,"ORG_REVIEW_APPLY",true,string(details));return out,nil
}
