package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
)

var (
	ErrReleaseNotFound=errors.New("release not found")
	ErrPreflight=errors.New("release preflight failed")
)

type Repository struct{DB *pgxpool.Pool}
type Manifest struct{ID int64 `json:"release_id"`;Version string `json:"version"`;BuildSHA string `json:"build_sha"`;RequiredSchemaVersion int64 `json:"required_schema_version"`;MapVersion string `json:"map_version,omitempty"`;WebIndexSchema int `json:"web_index_schema"`;OrganizationIndexSchema int `json:"organization_index_schema"`;AddressIndexSchema int `json:"address_index_schema"`;ConfigHash string `json:"config_hash"`;BackendImage string `json:"backend_image"`;FrontendImage string `json:"frontend_image"`;Status string `json:"status"`}
type Preflight struct{Pass bool `json:"pass"`;DatabaseSchema int64 `json:"database_schema"`;RequiredSchema int64 `json:"required_schema"`;ActiveMap string `json:"active_map,omitempty"`;RequiredMap string `json:"required_map,omitempty"`}

func (r Repository) Stage(ctx context.Context,version,buildSHA string,requiredSchema int64,mapVersion,configHash,backendImage,frontendImage,actor string)(Manifest,error){
	version=strings.TrimSpace(version);buildSHA=strings.TrimSpace(buildSHA);mapVersion=strings.TrimSpace(mapVersion);configHash=strings.ToLower(strings.TrimSpace(configHash));backendImage=strings.TrimSpace(backendImage);frontendImage=strings.TrimSpace(frontendImage);actor=strings.TrimSpace(actor)
	if r.DB==nil||version==""||len(version)>128||buildSHA==""||len(buildSHA)>128||requiredSchema<=0||len(configHash)!=64||backendImage==""||frontendImage==""||strings.ContainsAny(backendImage," \t\r\n")||strings.ContainsAny(frontendImage," \t\r\n")||actor==""{return Manifest{},ErrPreflight}
	var out Manifest
	err:=r.DB.QueryRow(ctx,`INSERT INTO app_releases(version,build_sha,required_schema_version,map_version,web_index_schema,organization_index_schema,address_index_schema,config_hash,backend_image,frontend_image)
VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10)
ON CONFLICT(version) DO UPDATE SET build_sha=EXCLUDED.build_sha,required_schema_version=EXCLUDED.required_schema_version,map_version=EXCLUDED.map_version,web_index_schema=EXCLUDED.web_index_schema,organization_index_schema=EXCLUDED.organization_index_schema,address_index_schema=EXCLUDED.address_index_schema,config_hash=EXCLUDED.config_hash,backend_image=EXCLUDED.backend_image,frontend_image=EXCLUDED.frontend_image
WHERE app_releases.status='STAGED'
RETURNING release_id,version,build_sha,required_schema_version,COALESCE(map_version,''),web_index_schema,organization_index_schema,address_index_schema,config_hash,COALESCE(backend_image,''),COALESCE(frontend_image,''),status`,version,buildSHA,requiredSchema,mapVersion,indexmanticore.WebSchemaVersion,indexmanticore.OrganizationsSchemaVersion,indexmanticore.AddressesSchemaVersion,configHash,backendImage,frontendImage).Scan(&out.ID,&out.Version,&out.BuildSHA,&out.RequiredSchemaVersion,&out.MapVersion,&out.WebIndexSchema,&out.OrganizationIndexSchema,&out.AddressIndexSchema,&out.ConfigHash,&out.BackendImage,&out.FrontendImage,&out.Status)
	if errors.Is(err,pgx.ErrNoRows){return Manifest{},ErrPreflight};if err!=nil{return Manifest{},err};_,_=r.DB.Exec(ctx,`INSERT INTO release_events(release_id,action,actor,details) VALUES($1,'STAGE',$2,jsonb_build_object('backend_image',$3,'frontend_image',$4))`,out.ID,actor,out.BackendImage,out.FrontendImage);return out,nil
}

func (r Repository) Preflight(ctx context.Context,version,actor string)(Preflight,error){
	manifest,err:=r.byVersion(ctx,version);if err!=nil{return Preflight{},err};var dbSchema int64;if err:=r.DB.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&dbSchema);err!=nil{return Preflight{},err};var activeMap string;_ = r.DB.QueryRow(ctx,`SELECT COALESCE(active_version,'') FROM map_state WHERE singleton=TRUE`).Scan(&activeMap)
	pass:=dbSchema>=manifest.RequiredSchemaVersion&&manifest.WebIndexSchema==indexmanticore.WebSchemaVersion&&manifest.OrganizationIndexSchema==indexmanticore.OrganizationsSchemaVersion&&manifest.AddressIndexSchema==indexmanticore.AddressesSchemaVersion&&(manifest.MapVersion==""||manifest.MapVersion==activeMap)&&manifest.BackendImage!=""&&manifest.FrontendImage!=""
	result:=Preflight{Pass:pass,DatabaseSchema:dbSchema,RequiredSchema:manifest.RequiredSchemaVersion,ActiveMap:activeMap,RequiredMap:manifest.MapVersion}
	if !pass{_,_=r.DB.Exec(ctx,`INSERT INTO release_events(release_id,action,actor,details) VALUES($1,'FAIL',$2,jsonb_build_object('database_schema',$3,'active_map',$4))`,manifest.ID,actor,dbSchema,activeMap);return result,ErrPreflight}
	if _,err=r.DB.Exec(ctx,`UPDATE app_releases SET preflight_at=now() WHERE release_id=$1`,manifest.ID);err!=nil{return result,err}
	if _,err=r.DB.Exec(ctx,`INSERT INTO release_events(release_id,action,actor,details) VALUES($1,'PREFLIGHT',$2,'{}')`,manifest.ID,actor);err!=nil{return result,err}
	return result,nil
}

func (r Repository) Activate(ctx context.Context,version,actor string)error{
	manifest,err:=r.byVersion(ctx,version);if err!=nil{return err};if manifest.Status!="STAGED"&&manifest.Status!="PREVIOUS"{return ErrPreflight};if _,err:=r.Preflight(ctx,version,actor);err!=nil{return err}
	tx,err:=r.DB.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var current *int64;if err:=tx.QueryRow(ctx,`SELECT active_release_id FROM release_state WHERE singleton=TRUE FOR UPDATE`).Scan(&current);err!=nil{return err};if current!=nil&&*current==manifest.ID{return tx.Commit(ctx)};if current!=nil{_,err=tx.Exec(ctx,`UPDATE app_releases SET status='PREVIOUS' WHERE release_id=$1`,*current);if err!=nil{return err}};_,err=tx.Exec(ctx,`UPDATE app_releases SET status='ACTIVE',activated_at=now() WHERE release_id=$1`,manifest.ID);if err!=nil{return err};_,err=tx.Exec(ctx,`UPDATE release_state SET previous_release_id=active_release_id,active_release_id=$1,updated_at=now() WHERE singleton=TRUE`,manifest.ID);if err!=nil{return err};_,err=tx.Exec(ctx,`INSERT INTO release_events(release_id,action,actor,details) VALUES($1,'ACTIVATE',$2,'{}')`,manifest.ID,actor);if err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) Rollback(ctx context.Context,actor string)error{
	tx,err:=r.DB.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var active,previous *int64;if err:=tx.QueryRow(ctx,`SELECT active_release_id,previous_release_id FROM release_state WHERE singleton=TRUE FOR UPDATE`).Scan(&active,&previous);err!=nil{return err};if active==nil||previous==nil{return ErrReleaseNotFound};var prevStatus string;if err:=tx.QueryRow(ctx,`SELECT status FROM app_releases WHERE release_id=$1 FOR UPDATE`,*previous).Scan(&prevStatus);err!=nil{return err};if prevStatus!="PREVIOUS"&&prevStatus!="STAGED"{return ErrPreflight};_,err=tx.Exec(ctx,`UPDATE app_releases SET status='PREVIOUS' WHERE release_id=$1`,*active);if err!=nil{return err};_,err=tx.Exec(ctx,`UPDATE app_releases SET status='ACTIVE',activated_at=now() WHERE release_id=$1`,*previous);if err!=nil{return err};_,err=tx.Exec(ctx,`UPDATE release_state SET active_release_id=$1,previous_release_id=$2,updated_at=now() WHERE singleton=TRUE`,*previous,*active);if err!=nil{return err};_,err=tx.Exec(ctx,`INSERT INTO release_events(release_id,action,actor,details) VALUES($1,'ROLLBACK',$2,jsonb_build_object('from_release_id',$3))`,*previous,actor,*active);if err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) Current(ctx context.Context)(active *Manifest,previous *Manifest,err error){
	var activeID,previousID *int64;if err=r.DB.QueryRow(ctx,`SELECT active_release_id,previous_release_id FROM release_state WHERE singleton=TRUE`).Scan(&activeID,&previousID);err!=nil{return nil,nil,err};if activeID!=nil{value,e:=r.byID(ctx,*activeID);if e!=nil{return nil,nil,e};active=&value};if previousID!=nil{value,e:=r.byID(ctx,*previousID);if e!=nil{return nil,nil,e};previous=&value};return
}
func (r Repository) byVersion(ctx context.Context,version string)(Manifest,error){var out Manifest;err:=r.DB.QueryRow(ctx,`SELECT release_id,version,build_sha,required_schema_version,COALESCE(map_version,''),web_index_schema,organization_index_schema,address_index_schema,config_hash,COALESCE(backend_image,''),COALESCE(frontend_image,''),status FROM app_releases WHERE version=$1`,strings.TrimSpace(version)).Scan(&out.ID,&out.Version,&out.BuildSHA,&out.RequiredSchemaVersion,&out.MapVersion,&out.WebIndexSchema,&out.OrganizationIndexSchema,&out.AddressIndexSchema,&out.ConfigHash,&out.BackendImage,&out.FrontendImage,&out.Status);if errors.Is(err,pgx.ErrNoRows){return Manifest{},ErrReleaseNotFound};if err!=nil{return Manifest{},fmt.Errorf("load release: %w",err)};return out,nil}
func (r Repository) byID(ctx context.Context,id int64)(Manifest,error){var out Manifest;err:=r.DB.QueryRow(ctx,`SELECT release_id,version,build_sha,required_schema_version,COALESCE(map_version,''),web_index_schema,organization_index_schema,address_index_schema,config_hash,COALESCE(backend_image,''),COALESCE(frontend_image,''),status FROM app_releases WHERE release_id=$1`,id).Scan(&out.ID,&out.Version,&out.BuildSHA,&out.RequiredSchemaVersion,&out.MapVersion,&out.WebIndexSchema,&out.OrganizationIndexSchema,&out.AddressIndexSchema,&out.ConfigHash,&out.BackendImage,&out.FrontendImage,&out.Status);if errors.Is(err,pgx.ErrNoRows){return Manifest{},ErrReleaseNotFound};return out,err}
