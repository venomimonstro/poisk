package maps

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrMapNotFound = errors.New("map version not found")
	ErrNoActiveMap = errors.New("active map is not configured")
	ErrNoRollback  = errors.New("previous map version is not available")
)

type Repository struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type State struct {
	ActiveVersion   string
	PreviousVersion string
	ActivatedAt     *time.Time
}

func (r *Repository) Register(ctx context.Context,m Manifest,actor string)error{
	if r==nil || r.db==nil{return errors.New("map repository is not initialized")}
	if err:=m.Validate();err!=nil{return err}
	if actor==""{actor="SYSTEM"}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	_,err=tx.Exec(ctx,`
INSERT INTO map_versions(
 map_version,pmtiles_path,style_path,pmtiles_sha256,style_sha256,pmtiles_size,style_size,
 min_lon,min_lat,max_lon,max_lat,min_zoom,max_zoom,center_lon,center_lat,center_zoom,source_name,attribution_html,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'VALIDATED')`,
		m.Version,m.PMTilesPath,m.StylePath,m.PMTilesSHA256,m.StyleSHA256,m.PMTilesSize,m.StyleSize,
		m.Bounds[0],m.Bounds[1],m.Bounds[2],m.Bounds[3],m.MinZoom,m.MaxZoom,m.Center[0],m.Center[1],m.Center[2],m.SourceName,m.AttributionHTML)
	if err!=nil{return fmt.Errorf("register map version: %w",err)}
	if _,err:=tx.Exec(ctx,`INSERT INTO map_version_events(action,map_version,actor) VALUES('REGISTER',$1,$2)`,m.Version,actor);err!=nil{return err}
	return tx.Commit(ctx)
}

func (r *Repository) Manifest(ctx context.Context,version string)(Manifest,error){
	if r==nil || r.db==nil{return Manifest{},errors.New("map repository is not initialized")}
	var m Manifest
	err:=r.db.QueryRow(ctx,`
SELECT map_version,pmtiles_path,style_path,pmtiles_sha256,style_sha256,pmtiles_size,style_size,
 min_lon,min_lat,max_lon,max_lat,min_zoom,max_zoom,center_lon,center_lat,center_zoom,source_name,attribution_html
FROM map_versions WHERE map_version=$1 AND status='VALIDATED'`,version).Scan(
		&m.Version,&m.PMTilesPath,&m.StylePath,&m.PMTilesSHA256,&m.StyleSHA256,&m.PMTilesSize,&m.StyleSize,
		&m.Bounds[0],&m.Bounds[1],&m.Bounds[2],&m.Bounds[3],&m.MinZoom,&m.MaxZoom,&m.Center[0],&m.Center[1],&m.Center[2],&m.SourceName,&m.AttributionHTML)
	if errors.Is(err,pgx.ErrNoRows){return Manifest{},ErrMapNotFound}
	if err!=nil{return Manifest{},fmt.Errorf("load map manifest: %w",err)}
	return m,nil
}

func (r *Repository) State(ctx context.Context)(State,error){
	var out State
	err:=r.db.QueryRow(ctx,`SELECT COALESCE(active_version,''),COALESCE(previous_version,''),activated_at FROM map_state WHERE singleton=TRUE`).Scan(&out.ActiveVersion,&out.PreviousVersion,&out.ActivatedAt)
	if errors.Is(err,pgx.ErrNoRows){return State{},ErrNoActiveMap}
	if err!=nil{return State{},err}
	if out.ActiveVersion==""{return out,ErrNoActiveMap}
	return out,nil
}

func (r *Repository) Activate(ctx context.Context,version,actor string)error{
	if actor==""{actor="SYSTEM"}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var exists bool
	if err:=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM map_versions WHERE map_version=$1 AND status='VALIDATED')`,version).Scan(&exists);err!=nil{return err}
	if !exists{return ErrMapNotFound}
	var current string
	if err:=tx.QueryRow(ctx,`SELECT COALESCE(active_version,'') FROM map_state WHERE singleton=TRUE FOR UPDATE`).Scan(&current);err!=nil{return err}
	if current==version{return nil}
	if _,err:=tx.Exec(ctx,`UPDATE map_state SET previous_version=NULLIF($1,''),active_version=$2,activated_at=now(),updated_at=now() WHERE singleton=TRUE`,current,version);err!=nil{return err}
	if _,err:=tx.Exec(ctx,`INSERT INTO map_version_events(action,map_version,from_version,actor) VALUES('ACTIVATE',$1,NULLIF($2,''),$3)`,version,current,actor);err!=nil{return err}
	return tx.Commit(ctx)
}

func (r *Repository) Rollback(ctx context.Context,actor string)(string,error){
	if actor==""{actor="SYSTEM"}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return "",err};defer func(){_=tx.Rollback(ctx)}()
	var active,previous string
	if err:=tx.QueryRow(ctx,`SELECT COALESCE(active_version,''),COALESCE(previous_version,'') FROM map_state WHERE singleton=TRUE FOR UPDATE`).Scan(&active,&previous);err!=nil{return "",err}
	if previous==""{return "",ErrNoRollback}
	var valid bool
	if err:=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM map_versions WHERE map_version=$1 AND status='VALIDATED')`,previous).Scan(&valid);err!=nil{return "",err}
	if !valid{return "",ErrNoRollback}
	if _,err:=tx.Exec(ctx,`UPDATE map_state SET active_version=$1,previous_version=NULLIF($2,''),activated_at=now(),updated_at=now() WHERE singleton=TRUE`,previous,active);err!=nil{return "",err}
	if _,err:=tx.Exec(ctx,`INSERT INTO map_version_events(action,map_version,from_version,actor) VALUES('ROLLBACK',$1,NULLIF($2,''),$3)`,previous,active,actor);err!=nil{return "",err}
	if err:=tx.Commit(ctx);err!=nil{return "",err}
	return previous,nil
}
