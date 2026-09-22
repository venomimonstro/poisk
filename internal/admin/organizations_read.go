package admin

import (
	"context"
	"errors"
	"strings"
	"time"
)

type OrganizationImportBatchRow struct {
	BatchID int64 `json:"batch_id"`
	SourceKey string `json:"source_key"`
	ExternalBatchKey string `json:"external_batch_key"`
	Mode string `json:"mode"`
	Status string `json:"status"`
	RowCount int64 `json:"row_count"`
	StagedCount int64 `json:"staged_count"`
	RejectedCount int64 `json:"rejected_count"`
	AppliedCount int64 `json:"applied_count"`
	CheckpointRow int64 `json:"checkpoint_row"`
	LastError string `json:"last_error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrganizationReviewRow struct {
	ReviewID int64 `json:"review_id"`
	BatchID int64 `json:"batch_id"`
	StagingID int64 `json:"staging_id"`
	CandidatePlaceID int64 `json:"candidate_place_id"`
	CandidateName string `json:"candidate_name"`
	IncomingName string `json:"incoming_name"`
	IncomingAddress string `json:"incoming_address,omitempty"`
	CandidateAddress string `json:"candidate_address,omitempty"`
	ReasonCode string `json:"reason_code"`
	Score int `json:"score"`
	Status string `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (s Service) ListOrganizationImports(ctx context.Context,session Session,status string,limit int)([]OrganizationImportBatchRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	status=strings.ToUpper(strings.TrimSpace(status));switch status{case "","STAGING","PLANNING","PLANNED","APPLYING","DONE","FAILED","CANCELLED":default:return nil,ErrInvalidCredential}
	if limit<=0{limit=50};if limit>200{limit=200}
	rows,err:=s.Store.db.Query(ctx,`SELECT batch_id,source_key,external_batch_key,mode,status,row_count,staged_count,rejected_count,applied_count,checkpoint_row,COALESCE(last_error,''),updated_at FROM organization_import_batches WHERE $1='' OR status=$1 ORDER BY updated_at DESC,batch_id DESC LIMIT $2`,status,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]OrganizationImportBatchRow,0,limit);for rows.Next(){var row OrganizationImportBatchRow;if err:=rows.Scan(&row.BatchID,&row.SourceKey,&row.ExternalBatchKey,&row.Mode,&row.Status,&row.RowCount,&row.StagedCount,&row.RejectedCount,&row.AppliedCount,&row.CheckpointRow,&row.LastError,&row.UpdatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func (s Service) ListOrganizationReviews(ctx context.Context,session Session,status string,limit int)([]OrganizationReviewRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	status=strings.ToUpper(strings.TrimSpace(status));switch status{case "","OPEN","MERGED","CREATE_NEW","REJECTED":default:return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>200{limit=200}
	rows,err:=s.Store.db.Query(ctx,`SELECT r.review_id,r.batch_id,r.staging_id,r.candidate_place_id,o.name,COALESCE(s.normalized_name,''),COALESCE(s.normalized_address,''),COALESCE(o.normalized_address,''),r.reason_code,r.score,r.status,r.created_at
FROM organization_merge_review r JOIN organization_staging_rows s ON s.staging_id=r.staging_id JOIN organizations o ON o.place_id=r.candidate_place_id
WHERE $1='' OR r.status=$1 ORDER BY CASE WHEN r.status='OPEN' THEN 0 ELSE 1 END,r.score DESC,r.created_at,r.review_id LIMIT $2`,status,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]OrganizationReviewRow,0,limit);for rows.Next(){var row OrganizationReviewRow;if err:=rows.Scan(&row.ReviewID,&row.BatchID,&row.StagingID,&row.CandidatePlaceID,&row.CandidateName,&row.IncomingName,&row.IncomingAddress,&row.CandidateAddress,&row.ReasonCode,&row.Score,&row.Status,&row.CreatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}
