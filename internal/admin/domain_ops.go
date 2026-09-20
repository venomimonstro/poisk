package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrPreviewInvalid = errors.New("admin preview invalid or expired")
	ErrDomainNotFound = errors.New("domain not found")
)

type DomainMutation struct {
	DomainID int64  `json:"domain_id"`
	Status   string `json:"status"`
	Policy   string `json:"policy"`
}

type DomainPreview struct {
	Token     string         `json:"preview_token"`
	ExpiresAt time.Time      `json:"expires_at"`
	Before    DomainMutation `json:"before"`
	After     DomainMutation `json:"after"`
	Host      string         `json:"host"`
}

func validDomainStatus(v string) bool {
	switch v { case "ACTIVE", "PAUSED", "DISABLED": return true }; return false
}
func validDomainPolicy(v string) bool {
	switch v { case "ALLOW", "LIMITED", "BLOCK", "REVIEW": return true }; return false
}

func (s Service) PreviewDomainMutation(ctx context.Context, session Session, domainID int64, status, policy string) (DomainPreview, error) {
	if s.Store == nil || domainID <= 0 { return DomainPreview{}, ErrInvalidCredential }
	if err := s.RequireRole(session, "OPERATOR"); err != nil { return DomainPreview{}, err }
	status = strings.ToUpper(strings.TrimSpace(status)); policy = strings.ToUpper(strings.TrimSpace(policy))
	if !validDomainStatus(status) || !validDomainPolicy(policy) { return DomainPreview{}, ErrInvalidCredential }

	var host, beforeStatus, beforePolicy string
	err := s.Store.db.QueryRow(ctx, `SELECT host,status,policy FROM domains WHERE domain_id=$1`, domainID).Scan(&host, &beforeStatus, &beforePolicy)
	if errors.Is(err, pgx.ErrNoRows) { return DomainPreview{}, ErrDomainNotFound }
	if err != nil { return DomainPreview{}, err }

	token, hash, err := RandomToken(24); if err != nil { return DomainPreview{}, err }
	expires := time.Now().Add(5 * time.Minute)
	payload, _ := json.Marshal(map[string]any{"status":status,"policy":policy,"host":host,"before_status":beforeStatus,"before_policy":beforePolicy})
	_, err = s.Store.db.Exec(ctx, `INSERT INTO admin_action_previews(admin_id,session_id,token_hash,action_type,target_type,target_id,payload,expires_at)
VALUES($1,$2,$3,'DOMAIN_POLICY','DOMAIN',$4,$5::jsonb,$6)`, session.AdminID, session.ID, hash, domainID, string(payload), expires)
	if err != nil { return DomainPreview{}, err }
	_ = s.Store.SecurityEvent(ctx, &session.AdminID, "DOMAIN_PREVIEW", true, string(payload))
	return DomainPreview{Token:token,ExpiresAt:expires,Host:host,Before:DomainMutation{DomainID:domainID,Status:beforeStatus,Policy:beforePolicy},After:DomainMutation{DomainID:domainID,Status:status,Policy:policy}}, nil
}

func (s Service) ApplyDomainMutation(ctx context.Context, session Session, token string) (DomainMutation, error) {
	if s.Store == nil || strings.TrimSpace(token)=="" { return DomainMutation{}, ErrPreviewInvalid }
	if err := s.RequireRole(session, "OPERATOR"); err != nil { return DomainMutation{}, err }
	hash := HashToken(token)
	tx, err := s.Store.db.BeginTx(ctx, pgx.TxOptions{}); if err != nil { return DomainMutation{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var previewID, domainID int64; var payloadRaw []byte
	err = tx.QueryRow(ctx, `SELECT preview_id,target_id,payload FROM admin_action_previews
WHERE admin_id=$1 AND session_id=$2 AND token_hash=$3 AND action_type='DOMAIN_POLICY' AND target_type='DOMAIN' AND consumed_at IS NULL AND expires_at>now()
FOR UPDATE`, session.AdminID, session.ID, hash).Scan(&previewID,&domainID,&payloadRaw)
	if errors.Is(err, pgx.ErrNoRows) { return DomainMutation{}, ErrPreviewInvalid }
	if err != nil { return DomainMutation{}, err }
	var payload struct{ Status string `json:"status"`; Policy string `json:"policy"` }
	if err := json.Unmarshal(payloadRaw,&payload); err != nil || !validDomainStatus(payload.Status) || !validDomainPolicy(payload.Policy) { return DomainMutation{}, ErrPreviewInvalid }
	var host string
	err = tx.QueryRow(ctx, `UPDATE domains SET status=$2,policy=$3,updated_at=now() WHERE domain_id=$1 RETURNING host`, domainID,payload.Status,payload.Policy).Scan(&host)
	if errors.Is(err, pgx.ErrNoRows) { return DomainMutation{}, ErrDomainNotFound }
	if err != nil { return DomainMutation{}, err }
	if _,err = tx.Exec(ctx, `UPDATE admin_action_previews SET consumed_at=now() WHERE preview_id=$1`, previewID); err != nil { return DomainMutation{}, err }
	details,_ := json.Marshal(map[string]any{"host":host,"status":payload.Status,"policy":payload.Policy,"preview_id":previewID})
	if _,err = tx.Exec(ctx, `INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details)
VALUES('ADMIN',$1::bigint::text,'DOMAIN_POLICY_APPLY','DOMAIN',$2::bigint::text,$3::jsonb)`, session.AdminID, domainID, string(details)); err != nil { return DomainMutation{}, err }
	if err = tx.Commit(ctx); err != nil { return DomainMutation{}, err }
	_ = s.Store.SecurityEvent(ctx,&session.AdminID,"DOMAIN_APPLY",true,string(details))
	return DomainMutation{DomainID:domainID,Status:payload.Status,Policy:payload.Policy}, nil
}
