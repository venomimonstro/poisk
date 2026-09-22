package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrQueryGapNotFound = errors.New("query gap not found")

type QueryGapMutation struct {
	GapID int64 `json:"gap_id"`
	State string `json:"state"`
}

type QueryGapPreview struct {
	Token string `json:"preview_token"`
	ExpiresAt time.Time `json:"expires_at"`
	RepresentativeQuery string `json:"representative_query,omitempty"`
	Before QueryGapMutation `json:"before"`
	After QueryGapMutation `json:"after"`
	Reason string `json:"reason"`
}

func (s Service) PreviewQueryGapMutation(ctx context.Context, session Session, gapID int64, action, reason string) (QueryGapPreview, error) {
	if s.Store == nil || gapID <= 0 { return QueryGapPreview{}, ErrInvalidCredential }
	if err := s.RequireRole(session, "OPERATOR"); err != nil { return QueryGapPreview{}, err }
	action = strings.ToUpper(strings.TrimSpace(action))
	reason = strings.TrimSpace(reason)
	if (action != "SUPPRESS" && action != "REOPEN") || len(reason) < 3 || len(reason) > 240 { return QueryGapPreview{}, ErrInvalidCredential }

	var state, query string
	var gapScore int
	err := s.Store.db.QueryRow(ctx, `SELECT state,COALESCE(representative_query,''),gap_score FROM query_gaps WHERE gap_id=$1`, gapID).Scan(&state,&query,&gapScore)
	if errors.Is(err, pgx.ErrNoRows) { return QueryGapPreview{}, ErrQueryGapNotFound }
	if err != nil { return QueryGapPreview{}, err }
	if action == "SUPPRESS" && state == "SUPPRESSED" { return QueryGapPreview{}, ErrInvalidCredential }
	if action == "REOPEN" && state != "SUPPRESSED" { return QueryGapPreview{}, ErrInvalidCredential }
	after := "SUPPRESSED"
	if action == "REOPEN" { after = "WATCH"; if gapScore >= 30 { after = "OPEN" } }

	token, hash, err := RandomToken(24); if err != nil { return QueryGapPreview{}, err }
	expires := time.Now().Add(5*time.Minute)
	payload, _ := json.Marshal(map[string]any{"action":action,"reason":reason,"before_state":state,"after_state":after})
	_, err = s.Store.db.Exec(ctx, `INSERT INTO admin_action_previews(admin_id,session_id,token_hash,action_type,target_type,target_id,payload,expires_at)
VALUES($1,$2,$3,'QUERY_GAP_STATE','QUERY_GAP',$4,$5::jsonb,$6)`, session.AdminID,session.ID,hash,gapID,string(payload),expires)
	if err != nil { return QueryGapPreview{}, err }
	_ = s.Store.SecurityEvent(ctx,&session.AdminID,"QUERY_GAP_PREVIEW",true,string(payload))
	return QueryGapPreview{Token:token,ExpiresAt:expires,RepresentativeQuery:query,Before:QueryGapMutation{GapID:gapID,State:state},After:QueryGapMutation{GapID:gapID,State:after},Reason:reason},nil
}

func (s Service) ApplyQueryGapMutation(ctx context.Context, session Session, token string) (QueryGapMutation,error) {
	if s.Store == nil || strings.TrimSpace(token)=="" { return QueryGapMutation{}, ErrPreviewInvalid }
	if err := s.RequireRole(session,"OPERATOR"); err != nil { return QueryGapMutation{}, err }
	hash := HashToken(token)
	tx, err := s.Store.db.BeginTx(ctx,pgx.TxOptions{}); if err != nil { return QueryGapMutation{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var previewID,gapID int64; var payloadRaw []byte
	err = tx.QueryRow(ctx, `SELECT preview_id,target_id,payload FROM admin_action_previews
WHERE admin_id=$1 AND session_id=$2 AND token_hash=$3 AND action_type='QUERY_GAP_STATE' AND target_type='QUERY_GAP' AND consumed_at IS NULL AND expires_at>now()
FOR UPDATE`,session.AdminID,session.ID,hash).Scan(&previewID,&gapID,&payloadRaw)
	if errors.Is(err,pgx.ErrNoRows) { return QueryGapMutation{}, ErrPreviewInvalid }
	if err != nil { return QueryGapMutation{}, err }
	var payload struct{ Action string `json:"action"`; Reason string `json:"reason"`; BeforeState string `json:"before_state"`; AfterState string `json:"after_state"` }
	if err:=json.Unmarshal(payloadRaw,&payload);err!=nil{return QueryGapMutation{},ErrPreviewInvalid}
	if (payload.Action!="SUPPRESS"&&payload.Action!="REOPEN")||len(payload.Reason)<3||len(payload.Reason)>240{return QueryGapMutation{},ErrPreviewInvalid}
	var current string; var gapScore int
	if err=tx.QueryRow(ctx,`SELECT state,gap_score FROM query_gaps WHERE gap_id=$1 FOR UPDATE`,gapID).Scan(&current,&gapScore);errors.Is(err,pgx.ErrNoRows){return QueryGapMutation{},ErrQueryGapNotFound}else if err!=nil{return QueryGapMutation{},err}
	if current!=payload.BeforeState{return QueryGapMutation{},ErrPreviewInvalid}
	newState:=payload.AfterState
	if payload.Action=="SUPPRESS" {
		newState="SUPPRESSED"
		if _,err=tx.Exec(ctx,`UPDATE query_gaps SET state='SUPPRESSED',suppressed_at=now() WHERE gap_id=$1`,gapID);err!=nil{return QueryGapMutation{},err}
		if _,err=tx.Exec(ctx,`DELETE FROM query_gap_domain_feedback WHERE gap_id=$1`,gapID);err!=nil{return QueryGapMutation{},err}
	} else {
		newState="WATCH";if gapScore>=30{newState="OPEN"}
		if _,err=tx.Exec(ctx,`UPDATE query_gaps SET state=$2,suppressed_at=NULL,last_feedback_at=NULL WHERE gap_id=$1`,gapID,newState);err!=nil{return QueryGapMutation{},err}
	}
	feedbackAction:="SUPPRESS";if payload.Action=="REOPEN"{feedbackAction="OPEN"}
	idempotencyKey:=fmt.Sprintf("admin-gap:%d:%s",previewID,strings.ToLower(payload.Action))
	if _,err=tx.Exec(ctx,`INSERT INTO query_gap_feedback_events(gap_id,action,idempotency_key,details)
VALUES($1,$2,$3,jsonb_build_object('reason',$4,'from_state',$5,'to_state',$6,'admin_id',$7))`,gapID,feedbackAction,idempotencyKey,payload.Reason,current,newState,session.AdminID);err!=nil{return QueryGapMutation{},err}
	if _,err=tx.Exec(ctx,`UPDATE admin_action_previews SET consumed_at=now() WHERE preview_id=$1`,previewID);err!=nil{return QueryGapMutation{},err}
	details,_:=json.Marshal(map[string]any{"action":payload.Action,"reason":payload.Reason,"from_state":current,"to_state":newState,"preview_id":previewID})
	if _,err=tx.Exec(ctx,`INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details)
VALUES('ADMIN',$1::bigint::text,'QUERY_GAP_STATE_APPLY','QUERY_GAP',$2::bigint::text,$3::jsonb)`,session.AdminID,gapID,string(details));err!=nil{return QueryGapMutation{},err}
	if err=tx.Commit(ctx);err!=nil{return QueryGapMutation{},err}
	_ = s.Store.SecurityEvent(ctx,&session.AdminID,"QUERY_GAP_APPLY",true,string(details))
	return QueryGapMutation{GapID:gapID,State:newState},nil
}
