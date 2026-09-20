package organizations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type applyRow struct {
	PlanID int64
	Action string
	TargetPlaceID *int64
	Staged StagedRow
}

func (r *Repository) ApplyNext(ctx context.Context,batchID int64,workerID string,leaseExtension time.Duration)(bool,error){
	if r==nil||r.db==nil{return false,errors.New("organization repository is not initialized")}
	if batchID<=0||workerID==""||leaseExtension<=0{return false,ErrInvalidBatch}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return false,err};defer func(){_=tx.Rollback(ctx)}()

	var checkpoint int64
	err=tx.QueryRow(ctx,`SELECT checkpoint_row FROM organization_import_batches
WHERE batch_id=$1 AND mode='APPLY' AND status='APPLYING' AND worker_id=$2 AND lease_until>now() FOR UPDATE`,batchID,workerID).Scan(&checkpoint)
	if errors.Is(err,pgx.ErrNoRows){return false,ErrLeaseLost}
	if err!=nil{return false,err}

	var item applyRow
	item.Staged.BatchID=batchID
	err=tx.QueryRow(ctx,`SELECT p.plan_id,p.action,p.target_place_id,
 s.staging_id,s.source_key,s.source_record_id,s.source_row_number,COALESCE(s.raw_payload->>'name',''),COALESCE(s.normalized_name,''),
 COALESCE(s.normalized_phone,''),COALESCE(s.normalized_website,''),COALESCE(s.raw_payload->>'address',''),COALESCE(s.normalized_address,''),
 COALESCE(s.category_key,''),s.latitude,s.longitude,s.payload_hash,s.state
FROM organization_import_plans p JOIN organization_staging_rows s ON s.staging_id=p.staging_id
WHERE p.batch_id=$1 AND p.plan_id>$2 AND p.applied_at IS NULL AND p.action IN ('CREATE','UPDATE','NOOP')
ORDER BY p.plan_id LIMIT 1 FOR UPDATE OF p,s`,batchID,checkpoint).Scan(
		&item.PlanID,&item.Action,&item.TargetPlaceID,&item.Staged.ID,&item.Staged.SourceKey,&item.Staged.SourceRecordID,&item.Staged.RowNumber,
		&item.Staged.Name,&item.Staged.NormalizedName,&item.Staged.Phone,&item.Staged.Website,&item.Staged.Address,&item.Staged.NormalizedAddress,
		&item.Staged.CategoryKey,&item.Staged.Latitude,&item.Staged.Longitude,&item.Staged.PayloadHash,&item.Staged.State)
	if errors.Is(err,pgx.ErrNoRows){
		var unresolved int
		if err:=tx.QueryRow(ctx,`SELECT count(*) FROM organization_import_plans WHERE batch_id=$1 AND action='REVIEW'`,batchID).Scan(&unresolved);err!=nil{return false,err}
		if unresolved>0{return false,ErrImportConflict}
		_,err=tx.Exec(ctx,`UPDATE organization_import_batches SET status='DONE',worker_id=NULL,lease_until=NULL,
 applied_count=(SELECT count(*) FROM organization_import_plans WHERE batch_id=$1 AND applied_at IS NOT NULL),updated_at=now()
WHERE batch_id=$1 AND worker_id=$2 AND status='APPLYING'`,batchID,workerID)
		if err!=nil{return false,err}
		if err:=tx.Commit(ctx);err!=nil{return false,err}
		return true,nil
	}
	if err!=nil{return false,err}

	lockKey:=item.Staged.SourceKey+"\x00"+item.Staged.SourceRecordID
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended($1,0))`,lockKey);err!=nil{return false,err}

	var placeID int64
	var version int64
	switch item.Action{
	case "NOOP":
		if err:=applyNoop(ctx,tx,item,&placeID);err!=nil{return false,err}
	case "CREATE":
		placeID,version,err=applyCreate(ctx,tx,item)
		if err!=nil{return false,err}
	case "UPDATE":
		if item.TargetPlaceID==nil{return false,ErrImportConflict}
		placeID,version,err=applyUpdate(ctx,tx,item,*item.TargetPlaceID)
		if err!=nil{return false,err}
	default:
		return false,ErrImportConflict
	}

	if version>0{
		_,err=tx.Exec(ctx,`INSERT INTO index_outbox(entity_type,entity_id,entity_version,operation,available_at)
VALUES('ORGANIZATION',$1,$2,'UPSERT',now()) ON CONFLICT(entity_type,entity_id,entity_version,operation) DO NOTHING`,placeID,version)
		if err!=nil{return false,fmt.Errorf("enqueue organization index event: %w",err)}
	}
	if _,err=tx.Exec(ctx,`UPDATE organization_import_plans SET applied_at=now() WHERE plan_id=$1 AND applied_at IS NULL`,item.PlanID);err!=nil{return false,err}
	if _,err=tx.Exec(ctx,`UPDATE organization_staging_rows SET state='APPLIED',updated_at=now() WHERE staging_id=$1`,item.Staged.ID);err!=nil{return false,err}
	tag,err:=tx.Exec(ctx,`UPDATE organization_import_batches SET checkpoint_row=$3,applied_count=applied_count+1,
 lease_until=now()+$4::interval,updated_at=now() WHERE batch_id=$1 AND worker_id=$2 AND status='APPLYING'`,batchID,workerID,item.PlanID,leaseExtension.String())
	if err!=nil{return false,err};if tag.RowsAffected()!=1{return false,ErrLeaseLost}
	if _,err=tx.Exec(ctx,`INSERT INTO organization_import_events(batch_id,staging_id,action,details)
VALUES($1,$2,'APPLY',jsonb_build_object('plan_id',$3,'place_id',$4,'action',$5))`,batchID,item.Staged.ID,item.PlanID,placeID,item.Action);err!=nil{return false,err}
	if err:=tx.Commit(ctx);err!=nil{return false,err}
	return false,nil
}

func applyNoop(ctx context.Context,tx pgx.Tx,item applyRow,placeID *int64)error{
	var storedHash string
	err:=tx.QueryRow(ctx,`UPDATE organization_source_links SET last_seen_at=now()
WHERE source_key=$1 AND source_record_id=$2 RETURNING place_id,source_payload_hash`,item.Staged.SourceKey,item.Staged.SourceRecordID).Scan(placeID,&storedHash)
	if errors.Is(err,pgx.ErrNoRows){return ErrImportConflict}
	if err!=nil{return err}
	if storedHash!=item.Staged.PayloadHash{return ErrImportConflict}
	return nil
}

func applyCreate(ctx context.Context,tx pgx.Tx,item applyRow)(int64,int64,error){
	var existingPlace int64
	var existingHash string
	err:=tx.QueryRow(ctx,`SELECT place_id,source_payload_hash FROM organization_source_links WHERE source_key=$1 AND source_record_id=$2 FOR UPDATE`,item.Staged.SourceKey,item.Staged.SourceRecordID).Scan(&existingPlace,&existingHash)
	if err==nil{
		if existingHash!=item.Staged.PayloadHash{return 0,0,ErrImportConflict}
		_,err=tx.Exec(ctx,`UPDATE organization_source_links SET last_seen_at=now() WHERE source_key=$1 AND source_record_id=$2`,item.Staged.SourceKey,item.Staged.SourceRecordID)
		return existingPlace,0,err
	}
	if !errors.Is(err,pgx.ErrNoRows){return 0,0,err}
	var placeID,version int64
	err=tx.QueryRow(ctx,`INSERT INTO organizations(name,normalized_name,category_key,phone,website,address,normalized_address,latitude,longitude,source_count)
VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,$9,1)
RETURNING place_id,version`,item.Staged.Name,item.Staged.NormalizedName,item.Staged.CategoryKey,item.Staged.Phone,item.Staged.Website,item.Staged.Address,item.Staged.NormalizedAddress,item.Staged.Latitude,item.Staged.Longitude).Scan(&placeID,&version)
	if err!=nil{return 0,0,err}
	_,err=tx.Exec(ctx,`INSERT INTO organization_source_links(source_key,source_record_id,place_id,source_payload_hash)
VALUES($1,$2,$3,$4)`,item.Staged.SourceKey,item.Staged.SourceRecordID,placeID,item.Staged.PayloadHash)
	if err!=nil{return 0,0,err}
	return placeID,version,nil
}

func applyUpdate(ctx context.Context,tx pgx.Tx,item applyRow,targetPlaceID int64)(int64,int64,error){
	var sourceCount int
	var currentPlace int64
	var linkedHash string
	linkErr:=tx.QueryRow(ctx,`SELECT place_id,source_payload_hash FROM organization_source_links
WHERE source_key=$1 AND source_record_id=$2 FOR UPDATE`,item.Staged.SourceKey,item.Staged.SourceRecordID).Scan(&currentPlace,&linkedHash)
	linked:=linkErr==nil
	if linkErr!=nil&&!errors.Is(linkErr,pgx.ErrNoRows){return 0,0,linkErr}
	if linked&&currentPlace!=targetPlaceID{return 0,0,ErrImportConflict}
	if linked&&linkedHash==item.Staged.PayloadHash{return targetPlaceID,0,nil}

	if err:=tx.QueryRow(ctx,`SELECT source_count FROM organizations WHERE place_id=$1 AND status IN ('ACTIVE','REVIEW') FOR UPDATE`,targetPlaceID).Scan(&sourceCount);errors.Is(err,pgx.ErrNoRows){return 0,0,ErrImportNotFound}else if err!=nil{return 0,0,err}
	if !linked{
		_,err:=tx.Exec(ctx,`INSERT INTO organization_source_links(source_key,source_record_id,place_id,source_payload_hash)
VALUES($1,$2,$3,$4)`,item.Staged.SourceKey,item.Staged.SourceRecordID,targetPlaceID,item.Staged.PayloadHash)
		if err!=nil{return 0,0,err}
		sourceCount++
	}else{
		_,err:=tx.Exec(ctx,`UPDATE organization_source_links SET source_payload_hash=$3,last_seen_at=now()
WHERE source_key=$1 AND source_record_id=$2`,item.Staged.SourceKey,item.Staged.SourceRecordID,item.Staged.PayloadHash)
		if err!=nil{return 0,0,err}
	}

	var version int64
	if linked&&sourceCount<=1{
		err:=tx.QueryRow(ctx,`UPDATE organizations SET version=version+1,name=$2,normalized_name=$3,category_key=NULLIF($4,''),
 phone=NULLIF($5,''),website=NULLIF($6,''),address=NULLIF($7,''),normalized_address=NULLIF($8,''),latitude=$9,longitude=$10,
 source_count=$11,updated_at=now() WHERE place_id=$1 RETURNING version`,targetPlaceID,item.Staged.Name,item.Staged.NormalizedName,item.Staged.CategoryKey,item.Staged.Phone,item.Staged.Website,item.Staged.Address,item.Staged.NormalizedAddress,item.Staged.Latitude,item.Staged.Longitude,sourceCount).Scan(&version)
		if err!=nil{return 0,0,err}
	}else{
		err:=tx.QueryRow(ctx,`UPDATE organizations SET version=version+1,
 category_key=COALESCE(category_key,NULLIF($2,'')),phone=COALESCE(phone,NULLIF($3,'')),website=COALESCE(website,NULLIF($4,'')),
 address=COALESCE(address,NULLIF($5,'')),normalized_address=COALESCE(normalized_address,NULLIF($6,'')),
 latitude=COALESCE(latitude,$7),longitude=COALESCE(longitude,$8),source_count=$9,updated_at=now()
WHERE place_id=$1 RETURNING version`,targetPlaceID,item.Staged.CategoryKey,item.Staged.Phone,item.Staged.Website,item.Staged.Address,item.Staged.NormalizedAddress,item.Staged.Latitude,item.Staged.Longitude,sourceCount).Scan(&version)
		if err!=nil{return 0,0,err}
	}
	return targetPlaceID,version,nil
}

type ApplyRunner struct {
	Store *Repository
	WorkerID string
	LeaseDuration time.Duration
	PollInterval time.Duration
}

func (r ApplyRunner) Run(ctx context.Context)error{
	if r.Store==nil||r.WorkerID==""{return ErrInvalidBatch}
	if r.LeaseDuration<=0{r.LeaseDuration=60*time.Second};if r.PollInterval<=0{r.PollInterval=time.Second}
	ticker:=time.NewTicker(r.PollInterval);defer ticker.Stop()
	for{
		if _,err:=r.Store.RequeueExpiredApply(ctx);err!=nil&&ctx.Err()==nil{return err}
		batch,err:=r.Store.LeaseReadyApply(ctx,r.WorkerID,r.LeaseDuration)
		if errors.Is(err,ErrImportNotFound){select{case<-ctx.Done():return ctx.Err();case<-ticker.C:continue}}
		if err!=nil{return err}
		for{
			done,err:=r.Store.ApplyNext(ctx,batch.ID,r.WorkerID,r.LeaseDuration)
			if err!=nil{return err};if done{break}
			if ctx.Err()!=nil{return ctx.Err()}
		}
	}
}
