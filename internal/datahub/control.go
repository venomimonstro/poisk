package datahub

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type snapshotHeader struct{
	Gate Gate `json:"gate"`
	CanonicalPath string `json:"canonical_path"`
	Title string `json:"title"`
	MetaDescription string `json:"meta_description"`
}

func (r *Repository) SuppressPage(ctx context.Context,pageID int64,reason string)error{
	if r==nil||r.db==nil||pageID<=0||len([]rune(reason))<2||len([]rune(reason))>160{return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var version int64;var state string
	if err=tx.QueryRow(ctx,`SELECT version,state FROM datahub_pages WHERE page_id=$1 FOR UPDATE`,pageID).Scan(&version,&state);errors.Is(err,pgx.ErrNoRows){return ErrInvalid};if err!=nil{return err}
	if _,err=tx.Exec(ctx,`UPDATE datahub_pages SET manual_suppressed=TRUE,state='SUPPRESSED',updated_at=now() WHERE page_id=$1`,pageID);err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason,details) VALUES($1,'SUPPRESS',$2,$2,$3,jsonb_build_object('from_state',$4))`,pageID,version,reason,state);err!=nil{return err};return tx.Commit(ctx)
}

func (r *Repository) ReopenPage(ctx context.Context,pageID int64,reason string)error{
	if r==nil||r.db==nil||pageID<=0||len([]rune(reason))<2||len([]rune(reason))>160{return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var version int64;var state string;var raw []byte
	if err=tx.QueryRow(ctx,`SELECT p.version,p.state,v.snapshot FROM datahub_pages p JOIN datahub_page_versions v ON v.page_id=p.page_id AND v.version=p.version WHERE p.page_id=$1 FOR UPDATE OF p`,pageID).Scan(&version,&state,&raw);errors.Is(err,pgx.ErrNoRows){return ErrInvalid};if err!=nil{return err}
	var h snapshotHeader;if err=json.Unmarshal(raw,&h);err!=nil{return err};newState:="DRAFT";if h.Gate.Publish{newState="PUBLISHED"}
	if _,err=tx.Exec(ctx,`UPDATE datahub_pages SET manual_suppressed=FALSE,state=$2,published_at=CASE WHEN $2='PUBLISHED' THEN COALESCE(published_at,now()) ELSE published_at END,updated_at=now() WHERE page_id=$1`,pageID,newState);err!=nil{return err}
	action:="BUILD";if newState=="PUBLISHED"{action="PUBLISH"};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason,details) VALUES($1,$2,$3,$3,$4,jsonb_build_object('from_state',$5,'manual_reopen',true))`,pageID,action,version,reason,state);err!=nil{return err};return tx.Commit(ctx)
}

func (r *Repository) RollbackPage(ctx context.Context,pageID,targetVersion int64,reason string,now time.Time)error{
	if r==nil||r.db==nil||pageID<=0||targetVersion<=0||len([]rune(reason))<2||len([]rune(reason))>160{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};now=now.UTC()
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var currentVersion int64;var canonical string
	if err=tx.QueryRow(ctx,`SELECT version,canonical_path FROM datahub_pages WHERE page_id=$1 FOR UPDATE`,pageID).Scan(&currentVersion,&canonical);errors.Is(err,pgx.ErrNoRows){return ErrInvalid};if err!=nil{return err}
	var hash string;var evidence,quality int;var raw []byte
	if err=tx.QueryRow(ctx,`SELECT content_hash,evidence_count,quality_score,snapshot FROM datahub_page_versions WHERE page_id=$1 AND version=$2`,pageID,targetVersion).Scan(&hash,&evidence,&quality,&raw);errors.Is(err,pgx.ErrNoRows){return ErrInvalid};if err!=nil{return err}
	var h snapshotHeader;if err=json.Unmarshal(raw,&h);err!=nil{return err};if h.CanonicalPath!=""&&h.CanonicalPath!=canonical{return ErrInvalid}
	var maxVersion int64;if err=tx.QueryRow(ctx,`SELECT max(version) FROM datahub_page_versions WHERE page_id=$1`,pageID).Scan(&maxVersion);err!=nil{return err};newVersion:=maxVersion+1;state:="DRAFT";if h.Gate.Publish{state="PUBLISHED"}
	if _,err=tx.Exec(ctx,`INSERT INTO datahub_page_versions(page_id,version,content_hash,evidence_count,quality_score,snapshot,created_at) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7)`,pageID,newVersion,hash,evidence,quality,string(raw),now);err!=nil{return err}
	if _,err=tx.Exec(ctx,`UPDATE datahub_pages SET version=$2,state=$3,manual_suppressed=FALSE,content_hash=$4,evidence_count=$5,quality_score=$6,title=$7,meta_description=$8,published_at=CASE WHEN $3='PUBLISHED' THEN COALESCE(published_at,$9) ELSE published_at END,refreshed_at=$9,updated_at=now() WHERE page_id=$1`,pageID,newVersion,state,hash,evidence,quality,h.Title,h.MetaDescription,now);err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason,details) VALUES($1,'ROLLBACK',$2,$3,$4,jsonb_build_object('source_version',$5))`,pageID,currentVersion,newVersion,reason,targetVersion);err!=nil{return err};return tx.Commit(ctx)
}
