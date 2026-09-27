package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type MailDNSHealth struct {
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

type MailGatewayHealth struct {
	ActiveAliases int64 `json:"active_aliases"`
	Ready int64 `json:"ready"`
	Leased int64 `json:"leased"`
	Retry int64 `json:"retry"`
	Submitted int64 `json:"submitted"`
	Delivered int64 `json:"delivered"`
	Bounced int64 `json:"bounced"`
	Dead int64 `json:"dead"`
	Inbound24h int64 `json:"inbound_24h"`
	Delivered24h int64 `json:"delivered_24h"`
	Bounced24h int64 `json:"bounced_24h"`
	Dead24h int64 `json:"dead_24h"`
	ActiveSuppressions int64 `json:"active_suppressions"`
	CoolingDomains int64 `json:"cooling_domains"`
	OldestQueuedSeconds int64 `json:"oldest_queued_seconds"`
	OldestSubmittedSeconds int64 `json:"oldest_submitted_seconds"`
	ReplayGuardRows int64 `json:"replay_guard_rows"`
	InboundReceiptsProcessing int64 `json:"inbound_receipts_processing"`
	DNS *MailDNSHealth `json:"dns,omitempty"`
	MeasuredAt time.Time `json:"measured_at"`
}

func (s Service) MailGatewayHealth(ctx context.Context,session Session)(MailGatewayHealth,error){
	if s.Store==nil||s.Store.db==nil{return MailGatewayHealth{},errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return MailGatewayHealth{},err}
	var out MailGatewayHealth
	err:=s.Store.db.QueryRow(ctx,`SELECT
 (SELECT count(*) FROM mail_external_aliases WHERE status='ACTIVE'),
 count(*) FILTER(WHERE status='READY'),
 count(*) FILTER(WHERE status='LEASED'),
 count(*) FILTER(WHERE status='RETRY'),
 count(*) FILTER(WHERE status='SUBMITTED'),
 count(*) FILTER(WHERE status='DELIVERED'),
 count(*) FILTER(WHERE status='BOUNCED'),
 count(*) FILTER(WHERE status='DEAD'),
 COALESCE(EXTRACT(EPOCH FROM (now()-min(created_at) FILTER(WHERE status IN ('READY','RETRY'))))::bigint,0),
 COALESCE(EXTRACT(EPOCH FROM (now()-min(submitted_at) FILTER(WHERE status='SUBMITTED')))::bigint,0)
 FROM mail_outbound_deliveries`).Scan(&out.ActiveAliases,&out.Ready,&out.Leased,&out.Retry,&out.Submitted,&out.Delivered,&out.Bounced,&out.Dead,&out.OldestQueuedSeconds,&out.OldestSubmittedSeconds);if err!=nil{return MailGatewayHealth{},err}
	if err=s.Store.db.QueryRow(ctx,`SELECT
 count(*) FILTER(WHERE direction='INBOUND' AND event_type='ACCEPT' AND created_at>=now()-interval '24 hours'),
 count(*) FILTER(WHERE direction='OUTBOUND' AND event_type='DELIVER' AND created_at>=now()-interval '24 hours'),
 count(*) FILTER(WHERE direction='OUTBOUND' AND event_type='BOUNCE' AND created_at>=now()-interval '24 hours'),
 (SELECT count(*) FROM mail_outbound_deliveries WHERE status='DEAD' AND updated_at>=now()-interval '24 hours'),
 (SELECT count(*) FROM mail_gateway_replay_guard WHERE expires_at>now()),
 (SELECT count(*) FROM mail_inbound_receipts WHERE status='PROCESSING'),
 (SELECT count(*) FROM mail_delivery_suppressions WHERE status='ACTIVE' AND suppressed_until>now()),
 (SELECT count(*) FROM mail_domain_delivery_pressure WHERE cooldown_until>now())
 FROM mail_gateway_events`).Scan(&out.Inbound24h,&out.Delivered24h,&out.Bounced24h,&out.Dead24h,&out.ReplayGuardRows,&out.InboundReceiptsProcessing,&out.ActiveSuppressions,&out.CoolingDomains);err!=nil{return MailGatewayHealth{},err}
	var dns MailDNSHealth;var reasonsJSON []byte
	err=s.Store.db.QueryRow(ctx,`SELECT domain,selector,mx_ok,spf_ok,dmarc_ok,dkim_ok,ready,drift,reasons,checked_at FROM mail_dns_readiness_snapshots ORDER BY checked_at DESC,snapshot_id DESC LIMIT 1`).Scan(&dns.Domain,&dns.Selector,&dns.MX,&dns.SPF,&dns.DMARC,&dns.DKIM,&dns.Ready,&dns.Drift,&reasonsJSON,&dns.CheckedAt)
	if err==nil{if len(reasonsJSON)>0{if decodeErr:=json.Unmarshal(reasonsJSON,&dns.Reasons);decodeErr!=nil{return MailGatewayHealth{},decodeErr}};out.DNS=&dns}else if !isNoRows(err){return MailGatewayHealth{},err}
	out.MeasuredAt=time.Now().UTC();return out,nil
}
