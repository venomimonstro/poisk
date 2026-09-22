package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type DataHubPageRow struct {
	PageID int64 `json:"page_id"`
	PageType string `json:"page_type"`
	CanonicalPath string `json:"canonical_path"`
	State string `json:"state"`
	Version int64 `json:"version"`
	EvidenceCount int `json:"evidence_count"`
	QualityScore int `json:"quality_score"`
	ManualSuppressed bool `json:"manual_suppressed"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DataHubPreview struct {
	Token string `json:"preview_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Page DataHubPageRow `json:"page"`
	Action string `json:"action"`
	TargetVersion int64 `json:"target_version,omitempty"`
	Reason string `json:"reason"`
}

func (s Service) ListDataHubPages(ctx context.Context,session Session,state string,limit int)([]DataHubPageRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	state=strings.ToUpper(strings.TrimSpace(state));if state!=""&&state!="DRAFT"&&state!="PUBLISHED"&&state!="SUPPRESSED"{return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>200{limit=200}
	rows,err:=s.Store.db.Query(ctx,`SELECT page_id,page_type,canonical_path,state,version,evidence_count,quality_score,manual_suppressed,updated_at FROM datahub_pages WHERE $1='' OR state=$1 ORDER BY updated_at DESC,page_id DESC LIMIT $2`,state,limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]DataHubPageRow,0,limit);for rows.Next(){var row DataHubPageRow;if err:=rows.Scan(&row.PageID,&row.PageType,&row.CanonicalPath,&row.State,&row.Version,&row.EvidenceCount,&row.QualityScore,&row.ManualSuppressed,&row.UpdatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func validDataHubAction(v string)bool{switch v{case "SUPPRESS","REOPEN","ROLLBACK":return true};return false}

func (s Service) PreviewDataHubMutation(ctx context.Context,session Session,pageID int64,action,reason string,targetVersion int64)(DataHubPreview,error){
	if s.Store==nil||s.Store.db==nil||pageID<=0{return DataHubPreview{},ErrInvalidCredential};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return DataHubPreview{},err}
	action=strings.ToUpper(strings.TrimSpace(action));reason=strings.TrimSpace(reason);if !validDataHubAction(action)||len([]rune(reason))<2||len([]rune(reason))>160{return DataHubPreview{},ErrInvalidCredential};if action=="ROLLBACK"&&targetVersion<=0{return DataHubPreview{},ErrInvalidCredential}
	var row DataHubPageRow;err:=s.Store.db.QueryRow(ctx,`SELECT page_id,page_type,canonical_path,state,version,evidence_count,quality_score,manual_suppressed,updated_at FROM datahub_pages WHERE page_id=$1`,pageID).Scan(&row.PageID,&row.PageType,&row.CanonicalPath,&row.State,&row.Version,&row.EvidenceCount,&row.QualityScore,&row.ManualSuppressed,&row.UpdatedAt);if errors.Is(err,pgx.ErrNoRows){return DataHubPreview{},ErrNotFound};if err!=nil{return DataHubPreview{},err}
	if action=="REOPEN"&&!row.ManualSuppressed{return DataHubPreview{},ErrInvalidCredential};if action=="ROLLBACK"{var exists bool;if err:=s.Store.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM datahub_page_versions WHERE page_id=$1 AND version=$2)`,pageID,targetVersion).Scan(&exists);err!=nil{return DataHubPreview{},err};if !exists{return DataHubPreview{},ErrInvalidCredential}}
	token,hash,err:=RandomToken(24);if err!=nil{return DataHubPreview{},err};expires:=time.Now().UTC().Add(5*time.Minute);payload,_:=json.Marshal(map[string]any{"action":action,"reason":reason,"target_version":targetVersion,"before_state":row.State,"before_version":row.Version,"canonical_path":row.CanonicalPath})
	if _,err=s.Store.db.Exec(ctx,`INSERT INTO admin_action_previews(admin_id,session_id,token_hash,action_type,target_type,target_id,payload,expires_at) VALUES($1,$2,$3,'DATAHUB_STATE','DATAHUB_PAGE',$4,$5::jsonb,$6)`,session.AdminID,session.ID,hash,pageID,string(payload),expires);err!=nil{return DataHubPreview{},err}
	_ = s.Store.SecurityEvent(ctx,&session.AdminID,"DATAHUB_PREVIEW",true,string(payload));return DataHubPreview{Token:token,ExpiresAt:expires,Page:row,Action:action,TargetVersion:targetVersion,Reason:reason},nil
}

func (s Service) ApplyDataHubMutation(ctx context.Context,session Session,token string)(DataHubPageRow,error){
	if s.Store==nil||s.Store.db==nil||strings.TrimSpace(token)==""{return DataHubPageRow{},ErrPreviewInvalid};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return DataHubPageRow{},err}
	tx,err:=s.Store.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return DataHubPageRow{},err};defer func(){_=tx.Rollback(ctx)}();hash:=HashToken(token);var previewID,pageID int64;var raw []byte
	err=tx.QueryRow(ctx,`SELECT preview_id,target_id,payload FROM admin_action_previews WHERE admin_id=$1 AND session_id=$2 AND token_hash=$3 AND action_type='DATAHUB_STATE' AND target_type='DATAHUB_PAGE' AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`,session.AdminID,session.ID,hash).Scan(&previewID,&pageID,&raw);if errors.Is(err,pgx.ErrNoRows){return DataHubPageRow{},ErrPreviewInvalid};if err!=nil{return DataHubPageRow{},err}
	var p struct{Action string `json:"action"`;Reason string `json:"reason"`;TargetVersion int64 `json:"target_version"`;BeforeVersion int64 `json:"before_version"`};if err:=json.Unmarshal(raw,&p);err!=nil||!validDataHubAction(p.Action){return DataHubPageRow{},ErrPreviewInvalid}
	var currentVersion int64;if err:=tx.QueryRow(ctx,`SELECT version FROM datahub_pages WHERE page_id=$1 FOR UPDATE`,pageID).Scan(&currentVersion);errors.Is(err,pgx.ErrNoRows){return DataHubPageRow{},ErrNotFound}else if err!=nil{return DataHubPageRow{},err};if currentVersion!=p.BeforeVersion{return DataHubPageRow{},ErrPreviewInvalid}
	switch p.Action{
	case "SUPPRESS":
		var state string;if err:=tx.QueryRow(ctx,`UPDATE datahub_pages SET manual_suppressed=TRUE,state='SUPPRESSED',updated_at=now() WHERE page_id=$1 RETURNING state`,pageID).Scan(&state);err!=nil{return DataHubPageRow{},err};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason,details) VALUES($1,'SUPPRESS',$2,$2,$3,jsonb_build_object('admin',true))`,pageID,currentVersion,p.Reason);err!=nil{return DataHubPageRow{},err}
	case "REOPEN":
		var rawSnapshot []byte;if err:=tx.QueryRow(ctx,`SELECT snapshot FROM datahub_page_versions WHERE page_id=$1 AND version=$2`,pageID,currentVersion).Scan(&rawSnapshot);err!=nil{return DataHubPageRow{},err};var h struct{Gate struct{Publish bool `json:"publish"`} `json:"gate"`};if err:=json.Unmarshal(rawSnapshot,&h);err!=nil{return DataHubPageRow{},err};state:="DRAFT";if h.Gate.Publish{state="PUBLISHED"};if _,err=tx.Exec(ctx,`UPDATE datahub_pages SET manual_suppressed=FALSE,state=$2,published_at=CASE WHEN $2='PUBLISHED' THEN COALESCE(published_at,now()) ELSE published_at END,updated_at=now() WHERE page_id=$1`,pageID,state);err!=nil{return DataHubPageRow{},err};action:="BUILD";if state=="PUBLISHED"{action="PUBLISH"};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason,details) VALUES($1,$2,$3,$3,$4,jsonb_build_object('admin',true,'manual_reopen',true))`,pageID,action,currentVersion,p.Reason);err!=nil{return DataHubPageRow{},err}
	case "ROLLBACK":
		var contentHash string;var evidence,quality int;var snapshot []byte;if err:=tx.QueryRow(ctx,`SELECT content_hash,evidence_count,quality_score,snapshot FROM datahub_page_versions WHERE page_id=$1 AND version=$2`,pageID,p.TargetVersion).Scan(&contentHash,&evidence,&quality,&snapshot);err!=nil{return DataHubPageRow{},ErrPreviewInvalid};var h struct{Gate struct{Publish bool `json:"publish"`} `json:"gate"`;Title string `json:"title"`;MetaDescription string `json:"meta_description"`;CanonicalPath string `json:"canonical_path"`};if err:=json.Unmarshal(snapshot,&h);err!=nil{return DataHubPageRow{},err};var canonical string;if err:=tx.QueryRow(ctx,`SELECT canonical_path FROM datahub_pages WHERE page_id=$1`,pageID).Scan(&canonical);err!=nil{return DataHubPageRow{},err};if h.CanonicalPath!=""&&h.CanonicalPath!=canonical{return DataHubPageRow{},ErrPreviewInvalid};var maxVersion int64;if err:=tx.QueryRow(ctx,`SELECT max(version) FROM datahub_page_versions WHERE page_id=$1`,pageID).Scan(&maxVersion);err!=nil{return DataHubPageRow{},err};newVersion:=maxVersion+1;state:="DRAFT";if h.Gate.Publish{state="PUBLISHED"};if _,err=tx.Exec(ctx,`INSERT INTO datahub_page_versions(page_id,version,content_hash,evidence_count,quality_score,snapshot,created_at) VALUES($1,$2,$3,$4,$5,$6::jsonb,now())`,pageID,newVersion,contentHash,evidence,quality,string(snapshot));err!=nil{return DataHubPageRow{},err};if _,err=tx.Exec(ctx,`UPDATE datahub_pages SET version=$2,state=$3,manual_suppressed=FALSE,content_hash=$4,evidence_count=$5,quality_score=$6,title=$7,meta_description=$8,published_at=CASE WHEN $3='PUBLISHED' THEN COALESCE(published_at,now()) ELSE published_at END,refreshed_at=now(),updated_at=now() WHERE page_id=$1`,pageID,newVersion,state,contentHash,evidence,quality,h.Title,h.MetaDescription);err!=nil{return DataHubPageRow{},err};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason,details) VALUES($1,'ROLLBACK',$2,$3,$4,jsonb_build_object('source_version',$5,'admin',true))`,pageID,currentVersion,newVersion,p.Reason,p.TargetVersion);err!=nil{return DataHubPageRow{},err}
	}
	if _,err=tx.Exec(ctx,`UPDATE admin_action_previews SET consumed_at=now() WHERE preview_id=$1`,previewID);err!=nil{return DataHubPageRow{},err};details,_:=json.Marshal(map[string]any{"preview_id":previewID,"action":p.Action,"target_version":p.TargetVersion});if _,err=tx.Exec(ctx,`INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details) VALUES('ADMIN',$1::bigint::text,'DATAHUB_STATE_APPLY','DATAHUB_PAGE',$2::bigint::text,$3::jsonb)`,session.AdminID,pageID,string(details));err!=nil{return DataHubPageRow{},err}
	var out DataHubPageRow;if err=tx.QueryRow(ctx,`SELECT page_id,page_type,canonical_path,state,version,evidence_count,quality_score,manual_suppressed,updated_at FROM datahub_pages WHERE page_id=$1`,pageID).Scan(&out.PageID,&out.PageType,&out.CanonicalPath,&out.State,&out.Version,&out.EvidenceCount,&out.QualityScore,&out.ManualSuppressed,&out.UpdatedAt);err!=nil{return DataHubPageRow{},err};if err=tx.Commit(ctx);err!=nil{return DataHubPageRow{},err};_ = s.Store.SecurityEvent(ctx,&session.AdminID,"DATAHUB_APPLY",true,string(details));return out,nil
}
