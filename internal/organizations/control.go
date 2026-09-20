package organizations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ImportSummary struct {
	Batch Batch `json:"batch"`
	Creates int64 `json:"creates"`
	Updates int64 `json:"updates"`
	Noops int64 `json:"noops"`
	Rejects int64 `json:"rejects"`
	Reviews int64 `json:"reviews"`
	OpenReviews int64 `json:"open_reviews"`
	Applied int64 `json:"applied"`
}

func (r *Repository) Summary(ctx context.Context,batchID int64)(ImportSummary,error){
	batch,err:=r.Batch(ctx,batchID);if err!=nil{return ImportSummary{},err}
	out:=ImportSummary{Batch:batch}
	err=r.db.QueryRow(ctx,`SELECT
 count(*) FILTER(WHERE action='CREATE'),count(*) FILTER(WHERE action='UPDATE'),count(*) FILTER(WHERE action='NOOP'),
 count(*) FILTER(WHERE action='REJECT'),count(*) FILTER(WHERE action='REVIEW'),count(*) FILTER(WHERE applied_at IS NOT NULL)
FROM organization_import_plans WHERE batch_id=$1`,batchID).Scan(&out.Creates,&out.Updates,&out.Noops,&out.Rejects,&out.Reviews,&out.Applied)
	if err!=nil{return ImportSummary{},err}
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM organization_merge_review WHERE batch_id=$1 AND status='OPEN'`,batchID).Scan(&out.OpenReviews);err!=nil{return ImportSummary{},err}
	return out,nil
}

func (r *Repository) PromoteDryRun(ctx context.Context,batchID int64)error{
	if r==nil||r.db==nil{return errors.New("organization repository is not initialized")}
	tag,err:=r.db.Exec(ctx,`UPDATE organization_import_batches b SET mode='APPLY',updated_at=now()
WHERE b.batch_id=$1 AND b.mode='DRY_RUN' AND b.status='PLANNED'
AND NOT EXISTS(SELECT 1 FROM organization_import_plans p WHERE p.batch_id=b.batch_id AND p.action='REVIEW')`,batchID)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrImportConflict};return nil
}

func (r *Repository) ResolveReview(ctx context.Context,stagingID int64,decision string,candidatePlaceID *int64,actor string)error{
	if r==nil||r.db==nil{return errors.New("organization repository is not initialized")}
	decision=strings.ToUpper(strings.TrimSpace(decision));actor=strings.TrimSpace(actor)
	if stagingID<=0||actor==""||(decision!="MERGE"&&decision!="CREATE_NEW"&&decision!="REJECT"){return ErrInvalidBatch}
	if decision=="MERGE"&&candidatePlaceID==nil{return ErrInvalidBatch}
	if decision!="MERGE"&&candidatePlaceID!=nil{return ErrInvalidBatch}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var planID,batchID int64
	err=tx.QueryRow(ctx,`SELECT p.plan_id,p.batch_id FROM organization_import_plans p
JOIN organization_import_batches b ON b.batch_id=p.batch_id
WHERE p.staging_id=$1 AND p.action='REVIEW' AND b.status='PLANNED' FOR UPDATE OF p,b`,stagingID).Scan(&planID,&batchID)
	if errors.Is(err,pgx.ErrNoRows){return ErrImportConflict};if err!=nil{return err}

	if decision=="MERGE"{
		var exists bool
		if err:=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_merge_review WHERE staging_id=$1 AND candidate_place_id=$2 AND status='OPEN')`,stagingID,*candidatePlaceID).Scan(&exists);err!=nil{return err}
		if !exists{return ErrImportConflict}
	}
	action:="CREATE";reason:=""
	if decision=="MERGE"{action="UPDATE"}
	if decision=="REJECT"{action="REJECT";reason="MANUAL_REJECT"}
	hashInput:=fmt.Sprintf("review:%d:%d:%s:%v",planID,stagingID,action,candidatePlaceID)
	h:=sha256.Sum256([]byte(hashInput));planHash:=hex.EncodeToString(h[:])
	_,err=tx.Exec(ctx,`UPDATE organization_import_plans SET action=$2,target_place_id=$3,match_rule='MANUAL_REVIEW',confidence=100,
 reason_code=NULLIF($4,''),plan_hash=$5 WHERE plan_id=$1`,planID,action,candidatePlaceID,reason,planHash)
	if err!=nil{return err}

	switch decision{
	case "MERGE":
		_,err=tx.Exec(ctx,`UPDATE organization_merge_review SET status=CASE WHEN candidate_place_id=$2 THEN 'MERGED' ELSE 'REJECTED' END,
 decided_by=$3,decided_at=now() WHERE staging_id=$1 AND status='OPEN'`,stagingID,*candidatePlaceID,actor)
	case "CREATE_NEW":
		_,err=tx.Exec(ctx,`UPDATE organization_merge_review SET status='CREATE_NEW',decided_by=$2,decided_at=now() WHERE staging_id=$1 AND status='OPEN'`,stagingID,actor)
	case "REJECT":
		_,err=tx.Exec(ctx,`UPDATE organization_merge_review SET status='REJECTED',decided_by=$2,decided_at=now() WHERE staging_id=$1 AND status='OPEN'`,stagingID,actor)
		if err==nil{
			_,err=tx.Exec(ctx,`UPDATE organization_staging_rows SET state='REJECTED',rejection_code='MANUAL_REJECT',rejection_detail='Rejected during duplicate review',updated_at=now() WHERE staging_id=$1`,stagingID)
		}
	}
	if err!=nil{return err}
	var candidateValue any
	if candidatePlaceID!=nil{candidateValue=*candidatePlaceID}
	if _,err=tx.Exec(ctx,`INSERT INTO organization_import_events(batch_id,staging_id,action,details)
VALUES($1,$2,'REVIEW_DECISION',jsonb_build_object('decision',$3,'actor',$4,'candidate_place_id',$5))`,batchID,stagingID,decision,actor,candidateValue);err!=nil{return err}
	return tx.Commit(ctx)
}

func (r *Repository) LeaseReadyApply(ctx context.Context,workerID string,lease time.Duration)(Batch,error){
	if workerID==""||lease<=0{return Batch{},ErrInvalidBatch}
	var b Batch
	err:=r.db.QueryRow(ctx,`WITH picked AS (
 SELECT b.batch_id FROM organization_import_batches b
 WHERE b.mode='APPLY' AND b.status='PLANNED'
 AND NOT EXISTS(SELECT 1 FROM organization_import_plans p WHERE p.batch_id=b.batch_id AND p.action='REVIEW')
 ORDER BY b.batch_id FOR UPDATE OF b SKIP LOCKED LIMIT 1
)
UPDATE organization_import_batches b SET status='APPLYING',worker_id=$1,lease_until=now()+$2::interval,updated_at=now()
FROM picked WHERE b.batch_id=picked.batch_id
RETURNING b.batch_id,b.source_key,b.external_batch_key,b.mode,b.status,b.checkpoint_row,COALESCE(b.worker_id,'')`,workerID,lease.String()).
		Scan(&b.ID,&b.SourceKey,&b.ExternalKey,&b.Mode,&b.Status,&b.Checkpoint,&b.WorkerID)
	if errors.Is(err,pgx.ErrNoRows){return Batch{},ErrImportNotFound}
	if err!=nil{return Batch{},err};return b,nil
}
