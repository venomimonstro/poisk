package address

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/address/gar"
)

var (
	ErrBatchConflict = errors.New("address import batch conflict")
	ErrBatchNotFound = errors.New("address import batch not found")
)

type Repository struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Batch struct {
	ID int64
	SourceRevision string
	Status string
	CheckpointFile string
	CheckpointRow int64
	StagedCount int64
	RejectedCount int64
	AppliedCount int64
}

func (r *Repository) CreateBatch(ctx context.Context,revision string)(Batch,error){
	if r==nil||r.db==nil{return Batch{},errors.New("address repository is not initialized")}
	revision=strings.TrimSpace(revision);if revision==""||len(revision)>128{return Batch{},gar.ErrInvalidRecord}
	var b Batch
	err:=r.db.QueryRow(ctx,`INSERT INTO address_import_batches(source_revision) VALUES($1)
ON CONFLICT(source_revision) DO UPDATE SET source_revision=EXCLUDED.source_revision
RETURNING batch_id,source_revision,status,COALESCE(checkpoint_file,''),checkpoint_row,staged_count,rejected_count,applied_count`,revision).
		Scan(&b.ID,&b.SourceRevision,&b.Status,&b.CheckpointFile,&b.CheckpointRow,&b.StagedCount,&b.RejectedCount,&b.AppliedCount)
	if err!=nil{return Batch{},fmt.Errorf("create address import batch: %w",err)}
	return b,nil
}

func (r *Repository) Batch(ctx context.Context,id int64)(Batch,error){
	var b Batch
	err:=r.db.QueryRow(ctx,`SELECT batch_id,source_revision,status,COALESCE(checkpoint_file,''),checkpoint_row,staged_count,rejected_count,applied_count FROM address_import_batches WHERE batch_id=$1`,id).
		Scan(&b.ID,&b.SourceRevision,&b.Status,&b.CheckpointFile,&b.CheckpointRow,&b.StagedCount,&b.RejectedCount,&b.AppliedCount)
	if errors.Is(err,pgx.ErrNoRows){return Batch{},ErrBatchNotFound};if err!=nil{return Batch{},err};return b,nil
}

func (r *Repository) Stage(ctx context.Context,batchID int64,sourceFile string,rowNumber int64,row gar.Normalized)(int64,error){
	sourceFile=strings.TrimSpace(sourceFile)
	if batchID<=0||sourceFile==""||len(sourceFile)>512||rowNumber<=0||row.ObjectID<=0||row.PayloadHash==""{return 0,gar.ErrInvalidRecord}
	var id int64
	err:=r.db.QueryRow(ctx,`WITH b AS (
 SELECT batch_id FROM address_import_batches WHERE batch_id=$1 AND status='STAGING'
), ins AS (
 INSERT INTO address_staging_rows(batch_id,source_file,source_row,record_kind,region_code,object_id,object_guid,parent_object_id,level,name,normalized_name,type_name,house_num,add_num1,add_num2,is_actual,is_active,payload_hash,raw_payload)
 SELECT b.batch_id,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,NULLIF($8,0),NULLIF($9,0),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),NULLIF($15,''),$16,$17,$18,$19::jsonb FROM b
 ON CONFLICT(batch_id,source_file,source_row) DO UPDATE SET updated_at=address_staging_rows.updated_at
 WHERE address_staging_rows.payload_hash=EXCLUDED.payload_hash
 RETURNING staging_id
)
SELECT staging_id FROM ins`,batchID,sourceFile,rowNumber,string(row.Kind),row.RegionCode,row.ObjectID,row.ObjectGUID,row.ParentObjectID,row.Level,row.Name,row.NormalizedName,row.TypeName,row.HouseNum,row.AddNum1,row.AddNum2,nullableBool(row.IsActual),nullableBool(row.IsActive),row.PayloadHash,string(row.RawPayload)).Scan(&id)
	if errors.Is(err,pgx.ErrNoRows)||isUnique(err){return 0,ErrBatchConflict}
	if err!=nil{return 0,fmt.Errorf("stage GAR row: %w",err)}
	return id,nil
}

func (r *Repository) Reject(ctx context.Context,batchID int64,sourceFile string,rowNumber int64,kind gar.Kind,region int,raw map[string]string,code,detail string)(int64,error){
	if batchID<=0||strings.TrimSpace(sourceFile)==""||rowNumber<=0||region<1||region>99{return 0,gar.ErrInvalidRecord}
	if kind!=gar.KindAddress&&kind!=gar.KindHouse&&kind!=gar.KindHierarchy{return 0,gar.ErrInvalidRecord}
	code=strings.TrimSpace(code);if code==""||len(code)>96{return 0,gar.ErrInvalidRecord}
	detail=truncateRunes(detail,1024)
	payload,err:=json.Marshal(raw);if err!=nil||len(payload)>64<<10{return 0,gar.ErrInvalidRecord}
	sum:=sha256.Sum256(payload);hash:=hex.EncodeToString(sum[:])
	var id int64
	err=r.db.QueryRow(ctx,`WITH b AS (SELECT batch_id FROM address_import_batches WHERE batch_id=$1 AND status='STAGING')
INSERT INTO address_import_rejections(batch_id,source_file,source_row,record_kind,region_code,error_code,error_detail,raw_payload,payload_hash)
SELECT b.batch_id,$2,$3,$4,$5,$6,NULLIF($7,''),$8::jsonb,$9 FROM b
ON CONFLICT(batch_id,source_file,source_row) DO UPDATE SET error_detail=address_import_rejections.error_detail
WHERE address_import_rejections.payload_hash=EXCLUDED.payload_hash AND address_import_rejections.error_code=EXCLUDED.error_code
RETURNING rejection_id`,batchID,sourceFile,rowNumber,string(kind),region,code,detail,string(payload),hash).Scan(&id)
	if errors.Is(err,pgx.ErrNoRows)||isUnique(err){return 0,ErrBatchConflict}
	if err!=nil{return 0,fmt.Errorf("reject GAR row: %w",err)}
	return id,nil
}

func (r *Repository) AdvanceCheckpoint(ctx context.Context,batchID int64,sourceFile string,row int64)error{
	if batchID<=0||strings.TrimSpace(sourceFile)==""||row<0{return gar.ErrInvalidRecord}
	tag,err:=r.db.Exec(ctx,`UPDATE address_import_batches SET checkpoint_file=$2,checkpoint_row=$3,
 staged_count=(SELECT count(*) FROM address_staging_rows WHERE batch_id=$1 AND state='VALID'),
 rejected_count=(SELECT count(*) FROM address_import_rejections WHERE batch_id=$1),updated_at=now()
WHERE batch_id=$1 AND status='STAGING'`,batchID,sourceFile,row)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrBatchConflict};return nil
}

func (r *Repository) FinishStaging(ctx context.Context,batchID int64)error{
	tag,err:=r.db.Exec(ctx,`UPDATE address_import_batches SET status='RESOLVING',updated_at=now() WHERE batch_id=$1 AND status='STAGING'`,batchID)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrBatchConflict};return nil
}

func nullableBool(value *bool)any{if value==nil{return nil};return *value}
func truncateRunes(value string,max int)string{value=strings.TrimSpace(value);if max<=0{return ""};if utf8.RuneCountInString(value)<=max{return value};r:=[]rune(value);return string(r[:max])}
func isUnique(err error)bool{var pgErr *pgconn.PgError;return errors.As(err,&pgErr)&&pgErr.Code=="23505"}
