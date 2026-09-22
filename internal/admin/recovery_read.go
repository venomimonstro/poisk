package admin

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReleaseSummary struct {
	Version string `json:"version"`
	BuildSHA string `json:"build_sha"`
	Status string `json:"status"`
	RequiredSchema int64 `json:"required_schema"`
	MapVersion string `json:"map_version,omitempty"`
	BackendImage string `json:"backend_image,omitempty"`
	FrontendImage string `json:"frontend_image,omitempty"`
}

type RecoveryDrillSummary struct {
	DrillID int64 `json:"drill_id"`
	Status string `json:"status"`
	ArtifactRef string `json:"artifact_ref,omitempty"`
	ArtifactBytes *int64 `json:"artifact_bytes,omitempty"`
	DurationMS *int64 `json:"duration_ms,omitempty"`
	DatabaseSchema int64 `json:"database_schema"`
	CompletedAt time.Time `json:"completed_at"`
}

type RecoveryReadiness struct {
	DatabaseSchema int64 `json:"database_schema"`
	Active *ReleaseSummary `json:"active_release,omitempty"`
	Previous *ReleaseSummary `json:"previous_release,omitempty"`
	Backup *RecoveryDrillSummary `json:"latest_backup,omitempty"`
	Restore *RecoveryDrillSummary `json:"latest_restore,omitempty"`
	Ready bool `json:"ready"`
	Reasons []string `json:"reasons,omitempty"`
}

func (s Service) RecoveryReadiness(ctx context.Context,session Session)(RecoveryReadiness,error){
	if s.Store==nil||s.Store.db==nil{return RecoveryReadiness{},errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return RecoveryReadiness{},err}
	var out RecoveryReadiness
	if err:=s.Store.db.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&out.DatabaseSchema);err!=nil{return RecoveryReadiness{},err}
	var activeID,previousID *int64
	if err:=s.Store.db.QueryRow(ctx,`SELECT active_release_id,previous_release_id FROM release_state WHERE singleton=TRUE`).Scan(&activeID,&previousID);err!=nil{return RecoveryReadiness{},err}
	loadRelease:=func(id *int64)(*ReleaseSummary,error){if id==nil{return nil,nil};var r ReleaseSummary;err:=s.Store.db.QueryRow(ctx,`SELECT version,build_sha,status,required_schema_version,COALESCE(map_version,''),COALESCE(backend_image,''),COALESCE(frontend_image,'') FROM app_releases WHERE release_id=$1`,*id).Scan(&r.Version,&r.BuildSHA,&r.Status,&r.RequiredSchema,&r.MapVersion,&r.BackendImage,&r.FrontendImage);if errors.Is(err,pgx.ErrNoRows){return nil,nil};return &r,err}
	var err error;if out.Active,err=loadRelease(activeID);err!=nil{return RecoveryReadiness{},err};if out.Previous,err=loadRelease(previousID);err!=nil{return RecoveryReadiness{},err}
	loadDrill:=func(kind string)(*RecoveryDrillSummary,error){var d RecoveryDrillSummary;err:=s.Store.db.QueryRow(ctx,`SELECT drill_id,status,COALESCE(artifact_ref,''),artifact_bytes,duration_ms,database_schema,completed_at FROM recovery_drills WHERE drill_type=$1 ORDER BY completed_at DESC,drill_id DESC LIMIT 1`,kind).Scan(&d.DrillID,&d.Status,&d.ArtifactRef,&d.ArtifactBytes,&d.DurationMS,&d.DatabaseSchema,&d.CompletedAt);if errors.Is(err,pgx.ErrNoRows){return nil,nil};return &d,err}
	if out.Backup,err=loadDrill("BACKUP");err!=nil{return RecoveryReadiness{},err};if out.Restore,err=loadDrill("RESTORE");err!=nil{return RecoveryReadiness{},err}
	if out.Active==nil{out.Reasons=append(out.Reasons,"no_active_release")}
	if out.Backup==nil{out.Reasons=append(out.Reasons,"no_backup_drill")}else if out.Backup.Status!="PASS"{out.Reasons=append(out.Reasons,"backup_drill_failed")}else if out.Backup.DatabaseSchema<out.DatabaseSchema{out.Reasons=append(out.Reasons,"backup_drill_schema_stale")}
	if out.Restore==nil{out.Reasons=append(out.Reasons,"no_restore_drill")}else if out.Restore.Status!="PASS"{out.Reasons=append(out.Reasons,"restore_drill_failed")}else if out.Restore.DatabaseSchema<out.DatabaseSchema{out.Reasons=append(out.Reasons,"restore_drill_schema_stale")}
	out.Ready=len(out.Reasons)==0
	return out,nil
}
