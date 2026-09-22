package recovery

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

type Drill struct {
	ID int64 `json:"drill_id"`
	Type string `json:"drill_type"`
	Status string `json:"status"`
	ArtifactRef string `json:"artifact_ref,omitempty"`
	ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
	ArtifactBytes *int64 `json:"artifact_bytes,omitempty"`
	DurationMS *int64 `json:"duration_ms,omitempty"`
	DatabaseSchema int64 `json:"database_schema"`
	Actor string `json:"actor"`
	CompletedAt time.Time `json:"completed_at"`
}

var ErrInvalid = errors.New("invalid recovery drill evidence")

func validType(v string) bool { return v=="BACKUP"||v=="RESTORE" }
func validStatus(v string) bool { return v=="PASS"||v=="FAIL" }

func (r Repository) Record(ctx context.Context, drillType,status,artifactRef,sha256 string,artifactBytes,durationMS *int64,actor string)(Drill,error){
	if r.DB==nil{return Drill{},ErrInvalid}
	drillType=strings.ToUpper(strings.TrimSpace(drillType));status=strings.ToUpper(strings.TrimSpace(status));artifactRef=strings.TrimSpace(artifactRef);sha256=strings.ToLower(strings.TrimSpace(sha256));actor=strings.TrimSpace(actor)
	if !validType(drillType)||!validStatus(status)||actor==""||len(actor)>120||len(artifactRef)>240{return Drill{},ErrInvalid}
	if sha256!=""&&len(sha256)!=64{return Drill{},ErrInvalid}
	if artifactBytes!=nil&&*artifactBytes<0{return Drill{},ErrInvalid};if durationMS!=nil&&*durationMS<0{return Drill{},ErrInvalid}
	var schema int64;if err:=r.DB.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&schema);err!=nil{return Drill{},err}
	var out Drill
	err:=r.DB.QueryRow(ctx,`INSERT INTO recovery_drills(drill_type,status,artifact_ref,artifact_sha256,artifact_bytes,duration_ms,database_schema,actor)
VALUES($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8)
RETURNING drill_id,drill_type,status,COALESCE(artifact_ref,''),COALESCE(artifact_sha256,''),artifact_bytes,duration_ms,database_schema,actor,completed_at`,drillType,status,artifactRef,sha256,artifactBytes,durationMS,schema,actor).Scan(&out.ID,&out.Type,&out.Status,&out.ArtifactRef,&out.ArtifactSHA256,&out.ArtifactBytes,&out.DurationMS,&out.DatabaseSchema,&out.Actor,&out.CompletedAt)
	return out,err
}

func (r Repository) Latest(ctx context.Context,drillType string)(*Drill,error){
	if r.DB==nil{return nil,ErrInvalid};drillType=strings.ToUpper(strings.TrimSpace(drillType));if !validType(drillType){return nil,ErrInvalid}
	var out Drill
	err:=r.DB.QueryRow(ctx,`SELECT drill_id,drill_type,status,COALESCE(artifact_ref,''),COALESCE(artifact_sha256,''),artifact_bytes,duration_ms,database_schema,actor,completed_at FROM recovery_drills WHERE drill_type=$1 ORDER BY completed_at DESC,drill_id DESC LIMIT 1`,drillType).Scan(&out.ID,&out.Type,&out.Status,&out.ArtifactRef,&out.ArtifactSHA256,&out.ArtifactBytes,&out.DurationMS,&out.DatabaseSchema,&out.Actor,&out.CompletedAt)
	if err!=nil{return nil,err};return &out,nil
}
