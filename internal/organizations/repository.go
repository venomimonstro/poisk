package organizations

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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrImportConflict = errors.New("organization import conflict")
	ErrImportNotFound = errors.New("organization import entity not found")
	ErrLeaseLost = errors.New("organization import lease lost")
)

type Repository struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Batch struct {
	ID int64
	SourceKey string
	ExternalKey string
	Mode string
	Status string
	Checkpoint int64
	WorkerID string
}

type StagedRow struct {
	ID int64
	BatchID int64
	SourceKey string
	SourceRecordID string
	RowNumber int64
	Name string
	NormalizedName string
	Phone string
	Website string
	Address string
	NormalizedAddress string
	CategoryKey string
	Latitude *float64
	Longitude *float64
	PayloadHash string
	State string
}

type Candidate struct {
	PlaceID int64
	Version int64
	NormalizedName string
	Phone string
	Website string
	NormalizedAddress string
	SourceCount int
}

func (r *Repository) EnsureSource(ctx context.Context,key,name string,trust int)error{
	if r==nil||r.db==nil{return errors.New("organization repository is not initialized")}
	key=strings.TrimSpace(key);name=compactText(name)
	if !ValidSourceKey(key)||name==""||trust<0||trust>100{return ErrInvalidBatch}
	_,err:=r.db.Exec(ctx,`INSERT INTO organization_sources(source_key,display_name,trust_level) VALUES($1,$2,$3)
ON CONFLICT(source_key) DO UPDATE SET display_name=EXCLUDED.display_name,trust_level=EXCLUDED.trust_level,updated_at=now()`,key,name,trust)
	if err!=nil{return fmt.Errorf("ensure organization source: %w",err)}
	return nil
}

func (r *Repository) CreateBatch(ctx context.Context,sourceKey,externalKey,mode string)(Batch,error){
	if r==nil||r.db==nil{return Batch{},errors.New("organization repository is not initialized")}
	sourceKey=strings.TrimSpace(sourceKey);externalKey=strings.TrimSpace(externalKey);mode=strings.ToUpper(strings.TrimSpace(mode))
	if !ValidSourceKey(sourceKey)||externalKey==""||len(externalKey)>256||(mode!="DRY_RUN"&&mode!="APPLY"){return Batch{},ErrInvalidBatch}
	var out Batch
	err:=r.db.QueryRow(ctx,`INSERT INTO organization_import_batches(source_key,external_batch_key,mode)
VALUES($1,$2,$3)
ON CONFLICT(source_key,external_batch_key) DO UPDATE SET source_key=EXCLUDED.source_key
RETURNING batch_id,source_key,external_batch_key,mode,status,checkpoint_row,COALESCE(worker_id,'')`,sourceKey,externalKey,mode).
		Scan(&out.ID,&out.SourceKey,&out.ExternalKey,&out.Mode,&out.Status,&out.Checkpoint,&out.WorkerID)
	if err!=nil{return Batch{},fmt.Errorf("create organization import batch: %w",err)}
	if out.Mode!=mode{return Batch{},ErrImportConflict}
	return out,nil
}

func (r *Repository) StageValid(ctx context.Context,batchID,rowNumber int64,row NormalizedRow)(int64,error){
	if batchID<=0||rowNumber<=0||row.SourceRecordID==""||row.PayloadHash==""||len(row.RawPayload)==0||len(row.RawPayload)>maxRawPayloadBytes{return 0,ErrInvalidRow}
	var id int64
	err:=r.db.QueryRow(ctx,`
WITH b AS (
 SELECT batch_id,source_key FROM organization_import_batches WHERE batch_id=$1 AND status='STAGING'
), ins AS (
 INSERT INTO organization_staging_rows(
  batch_id,source_key,source_record_id,source_row_number,raw_payload,raw_bytes,normalized_name,normalized_phone,
  normalized_website,normalized_address,category_key,latitude,longitude,state,payload_hash)
 SELECT b.batch_id,b.source_key,$2,$3,$4::jsonb,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,$12,'VALID',$13 FROM b
 ON CONFLICT(batch_id,source_record_id) DO UPDATE SET updated_at=organization_staging_rows.updated_at
 WHERE organization_staging_rows.payload_hash=EXCLUDED.payload_hash AND organization_staging_rows.source_row_number=EXCLUDED.source_row_number
 RETURNING staging_id
)
SELECT staging_id FROM ins`,batchID,row.SourceRecordID,rowNumber,string(row.RawPayload),len(row.RawPayload),row.NormalizedName,row.Phone,row.Website,row.NormalizedAddress,row.CategoryKey,row.Latitude,row.Longitude,row.PayloadHash).Scan(&id)
	if errors.Is(err,pgx.ErrNoRows){return 0,ErrImportConflict}
	if isUnique(err){return 0,ErrImportConflict}
	if err!=nil{return 0,fmt.Errorf("stage organization row: %w",err)}
	return id,nil
}

func (r *Repository) StageRejected(ctx context.Context,batchID,rowNumber int64,sourceRecordID string,raw []byte,code,detail string)(int64,error){
	if batchID<=0||rowNumber<=0||len(raw)==0||len(raw)>maxRawPayloadBytes{return 0,ErrInvalidRow}
	sourceRecordID=strings.TrimSpace(sourceRecordID);if sourceRecordID==""{sourceRecordID=fmt.Sprintf("invalid:%d",rowNumber)}
	if len(sourceRecordID)>256{return 0,ErrInvalidRow}
	code=strings.TrimSpace(code);if code==""||len(code)>96{return 0,ErrInvalidRow}
	if len(detail)>1024{detail=detail[:1024]}
	envelope,err:=json.Marshal(map[string]string{"raw":string(raw)});if err!=nil||len(envelope)>maxRawPayloadBytes{return 0,ErrInvalidRow}
	sum:=sha256.Sum256(raw);hash:=hex.EncodeToString(sum[:])
	var id int64
	err=r.db.QueryRow(ctx,`
WITH b AS (SELECT batch_id,source_key FROM organization_import_batches WHERE batch_id=$1 AND status='STAGING')
INSERT INTO organization_staging_rows(batch_id,source_key,source_record_id,source_row_number,raw_payload,raw_bytes,state,rejection_code,rejection_detail,payload_hash)
SELECT b.batch_id,b.source_key,$2,$3,$4::jsonb,$5,'REJECTED',$6,NULLIF($7,''),$8 FROM b
ON CONFLICT(batch_id,source_record_id) DO UPDATE SET updated_at=organization_staging_rows.updated_at
WHERE organization_staging_rows.payload_hash=EXCLUDED.payload_hash AND organization_staging_rows.source_row_number=EXCLUDED.source_row_number
RETURNING staging_id`,batchID,sourceRecordID,rowNumber,string(envelope),len(envelope),code,detail,hash).Scan(&id)
	if errors.Is(err,pgx.ErrNoRows)||isUnique(err){return 0,ErrImportConflict}
	if err!=nil{return 0,fmt.Errorf("stage rejected organization row: %w",err)}
	return id,nil
}

func (r *Repository) FinishStaging(ctx context.Context,batchID int64)error{
	tag,err:=r.db.Exec(ctx,`UPDATE organization_import_batches b SET status='PLANNING',
 row_count=s.total,staged_count=s.valid,rejected_count=s.rejected,updated_at=now()
FROM (SELECT batch_id,count(*) total,count(*) FILTER(WHERE state='VALID') valid,count(*) FILTER(WHERE state='REJECTED') rejected
      FROM organization_staging_rows WHERE batch_id=$1 GROUP BY batch_id) s
WHERE b.batch_id=s.batch_id AND b.batch_id=$1 AND b.status='STAGING'`,batchID)
	if err!=nil{return fmt.Errorf("finish organization staging: %w",err)}
	if tag.RowsAffected()!=1{return ErrImportConflict}
	return nil
}

func (r *Repository) RowsForPlanning(ctx context.Context,batchID int64,afterID int64,limit int)([]StagedRow,error){
	if limit<=0||limit>500{limit=100}
	rows,err:=r.db.Query(ctx,`SELECT staging_id,batch_id,source_key,source_record_id,source_row_number,
 COALESCE((raw_payload->>'name'),''),COALESCE(normalized_name,''),COALESCE(normalized_phone,''),COALESCE(normalized_website,''),
 COALESCE((raw_payload->>'address'),''),COALESCE(normalized_address,''),COALESCE(category_key,''),latitude,longitude,payload_hash,state
FROM organization_staging_rows WHERE batch_id=$1 AND staging_id>$2 AND state='VALID' ORDER BY staging_id LIMIT $3`,batchID,afterID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	out:=make([]StagedRow,0,limit)
	for rows.Next(){var v StagedRow;if err:=rows.Scan(&v.ID,&v.BatchID,&v.SourceKey,&v.SourceRecordID,&v.RowNumber,&v.Name,&v.NormalizedName,&v.Phone,&v.Website,&v.Address,&v.NormalizedAddress,&v.CategoryKey,&v.Latitude,&v.Longitude,&v.PayloadHash,&v.State);err!=nil{return nil,err};out=append(out,v)}
	return out,rows.Err()
}

func (r *Repository) LinkedCandidate(ctx context.Context,sourceKey,sourceRecordID string)(Candidate,bool,error){
	var c Candidate
	err:=r.db.QueryRow(ctx,`SELECT o.place_id,o.version,o.normalized_name,COALESCE(o.phone,''),COALESCE(o.website,''),COALESCE(o.normalized_address,''),o.source_count
FROM organization_source_links l JOIN organizations o ON o.place_id=l.place_id
WHERE l.source_key=$1 AND l.source_record_id=$2`,sourceKey,sourceRecordID).Scan(&c.PlaceID,&c.Version,&c.NormalizedName,&c.Phone,&c.Website,&c.NormalizedAddress,&c.SourceCount)
	if errors.Is(err,pgx.ErrNoRows){return Candidate{},false,nil}
	if err!=nil{return Candidate{},false,err}
	return c,true,nil
}

func (r *Repository) StrongCandidates(ctx context.Context,row StagedRow)([]Candidate,error){
	rows,err:=r.db.Query(ctx,`SELECT place_id,version,normalized_name,COALESCE(phone,''),COALESCE(website,''),COALESCE(normalized_address,''),source_count
FROM organizations
WHERE status IN ('ACTIVE','REVIEW') AND normalized_name=$1 AND (
 ($2<>'' AND phone=$2) OR ($3<>'' AND website=$3) OR ($4<>'' AND normalized_address=$4)
) ORDER BY place_id LIMIT 20`,row.NormalizedName,row.Phone,row.Website,row.NormalizedAddress)
	if err!=nil{return nil,err};defer rows.Close()
	var out []Candidate
	for rows.Next(){var c Candidate;if err:=rows.Scan(&c.PlaceID,&c.Version,&c.NormalizedName,&c.Phone,&c.Website,&c.NormalizedAddress,&c.SourceCount);err!=nil{return nil,err};out=append(out,c)}
	return out,rows.Err()
}

func (r *Repository) CompletePlanning(ctx context.Context,batchID int64)error{
	tag,err:=r.db.Exec(ctx,`UPDATE organization_import_batches SET status='PLANNED',updated_at=now() WHERE batch_id=$1 AND status='PLANNING'`,batchID)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrImportConflict};return nil
}

func (r *Repository) Batch(ctx context.Context,batchID int64)(Batch,error){
	var b Batch
	err:=r.db.QueryRow(ctx,`SELECT batch_id,source_key,external_batch_key,mode,status,checkpoint_row,COALESCE(worker_id,'') FROM organization_import_batches WHERE batch_id=$1`,batchID).
		Scan(&b.ID,&b.SourceKey,&b.ExternalKey,&b.Mode,&b.Status,&b.Checkpoint,&b.WorkerID)
	if errors.Is(err,pgx.ErrNoRows){return Batch{},ErrImportNotFound}
	if err!=nil{return Batch{},err};return b,nil
}

func (r *Repository) LeaseApply(ctx context.Context,workerID string,lease time.Duration)(Batch,error){
	if workerID==""||lease<=0{return Batch{},ErrInvalidBatch}
	var b Batch
	err:=r.db.QueryRow(ctx,`WITH picked AS (
 SELECT batch_id FROM organization_import_batches
 WHERE mode='APPLY' AND status='PLANNED' ORDER BY batch_id FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE organization_import_batches b SET status='APPLYING',worker_id=$1,lease_until=now()+$2::interval,updated_at=now()
FROM picked WHERE b.batch_id=picked.batch_id
RETURNING b.batch_id,b.source_key,b.external_batch_key,b.mode,b.status,b.checkpoint_row,COALESCE(b.worker_id,'')`,workerID,lease.String()).
		Scan(&b.ID,&b.SourceKey,&b.ExternalKey,&b.Mode,&b.Status,&b.Checkpoint,&b.WorkerID)
	if errors.Is(err,pgx.ErrNoRows){return Batch{},ErrImportNotFound}
	if err!=nil{return Batch{},err};return b,nil
}

func (r *Repository) RequeueExpiredApply(ctx context.Context)(int64,error){
	tag,err:=r.db.Exec(ctx,`UPDATE organization_import_batches SET status='PLANNED',worker_id=NULL,lease_until=NULL,last_error=COALESCE(last_error,'lease_expired'),updated_at=now()
WHERE status='APPLYING' AND lease_until<now()`)
	if err!=nil{return 0,err};return tag.RowsAffected(),nil
}

func isUnique(err error)bool{var pgErr *pgconn.PgError;return errors.As(err,&pgErr)&&pgErr.Code=="23505"}
