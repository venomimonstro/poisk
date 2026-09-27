package mail

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
)

type DNSReadinessSnapshot struct {
	ID int64 `json:"snapshot_id"`
	Domain string `json:"domain"`
	Selector string `json:"selector"`
	MX bool `json:"mx"`
	SPF bool `json:"spf"`
	DMARC bool `json:"dmarc"`
	DKIM bool `json:"dkim"`
	Ready bool `json:"ready"`
	Drift bool `json:"drift"`
	Reasons []string `json:"reasons,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

func readinessFingerprint(v DNSReadiness) string {
	reasons:=append([]string(nil),v.Reasons...)
	payload:=fmt.Sprintf("%t|%t|%t|%t|%t|%s",v.MX,v.SPF,v.DMARC,v.DKIM,v.Ready,strings.Join(reasons,","))
	sum:=sha256.Sum256([]byte(payload));return hex.EncodeToString(sum[:])
}

func (r Repository) RecordDNSReadiness(ctx context.Context, readiness DNSReadiness, checkedAt time.Time)(DNSReadinessSnapshot,error){
	if r.DB==nil{return DNSReadinessSnapshot{},ErrInvalid};readiness.Domain=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(readiness.Domain),"."));readiness.Selector=strings.ToLower(strings.TrimSpace(readiness.Selector));if readiness.Domain==""||readiness.Selector==""||len(readiness.Reasons)>8{return DNSReadinessSnapshot{},ErrInvalid};if checkedAt.IsZero(){checkedAt=time.Now().UTC()}
	for _,reason:=range readiness.Reasons{if len(reason)>80||strings.ContainsAny(reason,"\r\n\x00"){return DNSReadinessSnapshot{},ErrInvalid}}
	fingerprint:=readinessFingerprint(readiness);reasonsJSON,err:=json.Marshal(readiness.Reasons);if err!=nil{return DNSReadinessSnapshot{},err}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return DNSReadinessSnapshot{},err};defer func(){_=tx.Rollback(ctx)}()
	var previousFingerprint string
	err=tx.QueryRow(ctx,`SELECT fingerprint FROM mail_dns_readiness_snapshots WHERE domain=$1 AND selector=$2 ORDER BY checked_at DESC,snapshot_id DESC LIMIT 1`,readiness.Domain,readiness.Selector).Scan(&previousFingerprint)
	drift:=false;if err==nil{drift=previousFingerprint!=fingerprint}else if !errors.Is(err,pgx.ErrNoRows){return DNSReadinessSnapshot{},err}
	var out DNSReadinessSnapshot
	err=tx.QueryRow(ctx,`INSERT INTO mail_dns_readiness_snapshots(domain,selector,mx_ok,spf_ok,dmarc_ok,dkim_ok,ready,drift,fingerprint,reasons,checked_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11)
RETURNING snapshot_id,domain,selector,mx_ok,spf_ok,dmarc_ok,dkim_ok,ready,drift,reasons,checked_at`,readiness.Domain,readiness.Selector,readiness.MX,readiness.SPF,readiness.DMARC,readiness.DKIM,readiness.Ready,drift,fingerprint,string(reasonsJSON),checkedAt).Scan(&out.ID,&out.Domain,&out.Selector,&out.MX,&out.SPF,&out.DMARC,&out.DKIM,&out.Ready,&out.Drift,&reasonsJSON,&out.CheckedAt);if err!=nil{return DNSReadinessSnapshot{},err}
	if len(reasonsJSON)>0{if err=json.Unmarshal(reasonsJSON,&out.Reasons);err!=nil{return DNSReadinessSnapshot{},err}}
	if err=tx.Commit(ctx);err!=nil{return DNSReadinessSnapshot{},err};return out,nil
}

func (r Repository) LatestDNSReadiness(ctx context.Context,domain,selector string)(*DNSReadinessSnapshot,error){
	if r.DB==nil{return nil,ErrInvalid};domain=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain),"."));selector=strings.ToLower(strings.TrimSpace(selector));if domain==""||selector==""{return nil,ErrInvalid}
	var out DNSReadinessSnapshot;var reasonsJSON []byte
	err:=r.DB.QueryRow(ctx,`SELECT snapshot_id,domain,selector,mx_ok,spf_ok,dmarc_ok,dkim_ok,ready,drift,reasons,checked_at FROM mail_dns_readiness_snapshots WHERE domain=$1 AND selector=$2 ORDER BY checked_at DESC,snapshot_id DESC LIMIT 1`,domain,selector).Scan(&out.ID,&out.Domain,&out.Selector,&out.MX,&out.SPF,&out.DMARC,&out.DKIM,&out.Ready,&out.Drift,&reasonsJSON,&out.CheckedAt)
	if errors.Is(err,pgx.ErrNoRows){return nil,nil};if err!=nil{return nil,err};if len(reasonsJSON)>0{if err=json.Unmarshal(reasonsJSON,&out.Reasons);err!=nil{return nil,err}};return &out,nil
}
