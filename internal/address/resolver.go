package address

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ResolveResult struct {
	Applied  int64 `json:"applied"`
	Orphans  int64 `json:"orphans"`
	Passes   int   `json:"passes"`
}

// ResolveBatch builds the canonical hierarchy only from staged GAR rows.
// A row is applied when its parent is either absent (root) or already canonical.
// Remaining rows after a no-progress pass are treated as orphan/cycle diagnostics.
func (r *Repository) ResolveBatch(ctx context.Context,batchID int64)(ResolveResult,error){
	if r==nil||r.db==nil{return ResolveResult{},errors.New("address repository is not initialized")}
	if batchID<=0{return ResolveResult{},ErrBatchConflict}
	batch,err:=r.Batch(ctx,batchID);if err!=nil{return ResolveResult{},err}
	if batch.Status!="RESOLVING"&&batch.Status!="APPLYING"{return ResolveResult{},ErrBatchConflict}
	if batch.Status=="RESOLVING"{
		tag,err:=r.db.Exec(ctx,`UPDATE address_import_batches SET status='APPLYING',updated_at=now() WHERE batch_id=$1 AND status='RESOLVING'`,batchID)
		if err!=nil{return ResolveResult{},err};if tag.RowsAffected()!=1{return ResolveResult{},ErrBatchConflict}
	}
	result:=ResolveResult{}
	for pass:=1;pass<=128;pass++{
		if err:=ctx.Err();err!=nil{return result,err}
		applied,err:=r.resolvePass(ctx,batchID,batch.SourceRevision)
		if err!=nil{return result,err}
		result.Passes=pass;result.Applied+=applied
		if applied==0{break}
	}
	var remaining int64
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM address_staging_rows WHERE batch_id=$1 AND state='VALID' AND record_kind IN ('ADDR_OBJ','HOUSE')`,batchID).Scan(&remaining);err!=nil{return result,err}
	result.Orphans=remaining
	if remaining>0{
		_,err:=r.db.Exec(ctx,`INSERT INTO address_import_events(batch_id,action,details)
VALUES($1,'ORPHAN_SUMMARY',jsonb_build_object('remaining',$2))`,batchID,remaining)
		if err!=nil{return result,err}
	}
	_,err=r.db.Exec(ctx,`UPDATE address_import_batches SET status='DONE',applied_count=(SELECT count(*) FROM address_staging_rows WHERE batch_id=$1 AND state='APPLIED'),
 rejected_count=(SELECT count(*) FROM address_import_rejections WHERE batch_id=$1)+$2,updated_at=now(),last_error=CASE WHEN $2>0 THEN 'orphan_or_cycle_rows' ELSE NULL END
WHERE batch_id=$1 AND status='APPLYING'`,batchID,remaining)
	if err!=nil{return result,err}
	return result,nil
}

func (r *Repository) resolvePass(ctx context.Context,batchID int64,revision string)(int64,error){
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,err};defer func(){_=tx.Rollback(ctx)}()
	const q=`
WITH candidates AS (
 SELECT s.staging_id,s.region_code,s.object_id,s.object_guid,s.record_kind,s.level,s.name,s.normalized_name,s.type_name,s.house_num,
        h.parent_object_id,
        p.address_id AS parent_address_id,p.full_address AS parent_full
 FROM address_staging_rows s
 LEFT JOIN address_staging_rows h ON h.batch_id=s.batch_id AND h.record_kind='HIERARCHY' AND h.region_code=s.region_code AND h.object_id=s.object_id AND h.state='VALID'
 LEFT JOIN addresses p ON p.region_code=s.region_code AND p.gar_object_id=h.parent_object_id
 WHERE s.batch_id=$1 AND s.state='VALID' AND s.record_kind IN ('ADDR_OBJ','HOUSE')
   AND COALESCE(s.is_actual,TRUE)=TRUE AND COALESCE(s.is_active,TRUE)=TRUE
   AND (h.parent_object_id IS NULL OR p.address_id IS NOT NULL)
 ORDER BY CASE WHEN s.record_kind='ADDR_OBJ' THEN 0 ELSE 1 END,s.level NULLS FIRST,s.staging_id
 LIMIT 5000
), applied AS (
 INSERT INTO addresses(region_code,gar_object_id,object_guid,object_kind,level,parent_address_id,name,type_name,house_num,normalized_name,display_name,full_address,status,source_revision)
 SELECT c.region_code,c.object_id,c.object_guid,c.record_kind,c.level,c.parent_address_id,
        CASE WHEN c.record_kind='HOUSE' THEN COALESCE(NULLIF(c.house_num,''),c.name,'дом') ELSE COALESCE(NULLIF(c.name,''),'без названия') END,
        NULLIF(c.type_name,''),NULLIF(c.house_num,''),
        COALESCE(NULLIF(c.normalized_name,''),lower(COALESCE(NULLIF(c.house_num,''),c.name,'дом'))),
        CASE WHEN c.record_kind='HOUSE' THEN trim(concat_ws(' ','д.',NULLIF(c.house_num,''))) ELSE trim(concat_ws(' ',NULLIF(c.type_name,''),NULLIF(c.name,''))) END,
        trim(concat_ws(', ',NULLIF(c.parent_full,''),CASE WHEN c.record_kind='HOUSE' THEN trim(concat_ws(' ','д.',NULLIF(c.house_num,''))) ELSE trim(concat_ws(' ',NULLIF(c.type_name,''),NULLIF(c.name,''))) END)),
        'ACTIVE',$2
 FROM candidates c
 ON CONFLICT(region_code,gar_object_id) DO UPDATE SET
   version=addresses.version+1,object_guid=COALESCE(EXCLUDED.object_guid,addresses.object_guid),object_kind=EXCLUDED.object_kind,level=EXCLUDED.level,
   parent_address_id=EXCLUDED.parent_address_id,name=EXCLUDED.name,type_name=EXCLUDED.type_name,house_num=EXCLUDED.house_num,
   normalized_name=EXCLUDED.normalized_name,display_name=EXCLUDED.display_name,full_address=EXCLUDED.full_address,status='ACTIVE',source_revision=EXCLUDED.source_revision,updated_at=now()
 RETURNING address_id,version,region_code,gar_object_id
), marked AS (
 UPDATE address_staging_rows s SET state='APPLIED',updated_at=now()
 FROM candidates c WHERE s.staging_id=c.staging_id
 RETURNING s.staging_id,s.region_code,s.object_id
), queued AS (
 INSERT INTO index_outbox(entity_type,entity_id,entity_version,operation,available_at)
 SELECT 'ADDRESS',a.address_id,a.version,'UPSERT',now() FROM applied a
 ON CONFLICT(entity_type,entity_id,entity_version) DO NOTHING
 RETURNING id
)
SELECT count(*) FROM marked`
	var count int64
	if err:=tx.QueryRow(ctx,q,batchID,strings.TrimSpace(revision)).Scan(&count);err!=nil{return 0,fmt.Errorf("resolve address pass: %w",err)}
	if count>0{
		if _,err:=tx.Exec(ctx,`INSERT INTO address_import_events(batch_id,action,details) VALUES($1,'RESOLVE_PASS',jsonb_build_object('applied',$2))`,batchID,count);err!=nil{return 0,err}
	}
	if err:=tx.Commit(ctx);err!=nil{return 0,err}
	return count,nil
}
