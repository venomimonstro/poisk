package mail

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	staleSubmittedAfter = 24 * time.Hour
	gatewayEventRetention = 90 * 24 * time.Hour
	outboundEventRetention = 180 * 24 * time.Hour
	dnsSnapshotRetention = 90 * 24 * time.Hour
	pressureRetention = 7 * 24 * time.Hour
)

type DeliverabilityMaintenance struct {
	StaleSubmitted int64 `json:"stale_submitted"`
	ReplayDeleted int64 `json:"replay_deleted"`
	GatewayEventsDeleted int64 `json:"gateway_events_deleted"`
	OutboundEventsDeleted int64 `json:"outbound_events_deleted"`
	DNSSnapshotsDeleted int64 `json:"dns_snapshots_deleted"`
	PressureRowsDeleted int64 `json:"pressure_rows_deleted"`
}

func (r Repository) MaintainDeliverability(ctx context.Context,now time.Time)(DeliverabilityMaintenance,error){
	if r.DB==nil{return DeliverabilityMaintenance{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()}
	tx,err:=r.DB.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return DeliverabilityMaintenance{},err};defer func(){_=tx.Rollback(ctx)}()
	var out DeliverabilityMaintenance
	rows,err:=tx.Query(ctx,`UPDATE mail_outbound_deliveries
SET status='DEAD',last_error_code='SUBMITTED_STALE',last_error_detail=NULL,updated_at=$1
WHERE status='SUBMITTED' AND submitted_at<$2
RETURNING delivery_id,attempts`,now,now.Add(-staleSubmittedAfter));if err!=nil{return out,err}
	type staleRow struct{id int64;attempt int};stale:=[]staleRow{}
	for rows.Next(){var x staleRow;if err=rows.Scan(&x.id,&x.attempt);err!=nil{rows.Close();return out,err};stale=append(stale,x)};if err=rows.Err();err!=nil{rows.Close();return out,err};rows.Close();out.StaleSubmitted=int64(len(stale))
	for _,x:=range stale{
		if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'DEAD',$2,jsonb_build_object('code','SUBMITTED_STALE'))`,x.id,x.attempt);err!=nil{return out,err}
		if _,err=tx.Exec(ctx,`INSERT INTO mail_gateway_events(direction,event_type,delivery_id,code,details) VALUES('OUTBOUND','DEFER',$1,'SUBMITTED_STALE',jsonb_build_object('reason','callback_timeout'))`,x.id);err!=nil{return out,err}
	}
	if out.ReplayDeleted,err=deleteBounded(ctx,tx,`mail_gateway_replay_guard`,`event_id`,`expires_at<=$1`,now,2000);err!=nil{return out,err}
	if out.GatewayEventsDeleted,err=deleteBounded(ctx,tx,`mail_gateway_events`,`gateway_event_id`,`created_at<$1`,now.Add(-gatewayEventRetention),5000);err!=nil{return out,err}
	// DEAD_RETRY is retained because it is the durable authorization/rate-limit audit
	// for manual retries. There can be at most MaxManualDeadRetries per delivery.
	if out.OutboundEventsDeleted,err=deleteBounded(ctx,tx,`mail_outbound_events`,`event_id`,`created_at<$1 AND action<>'DEAD_RETRY'`,now.Add(-outboundEventRetention),5000);err!=nil{return out,err}
	if out.DNSSnapshotsDeleted,err=deleteBounded(ctx,tx,`mail_dns_readiness_snapshots`,`snapshot_id`,`checked_at<$1`,now.Add(-dnsSnapshotRetention),2000);err!=nil{return out,err}
	if out.PressureRowsDeleted,err=deleteBounded(ctx,tx,`mail_domain_delivery_pressure`,`domain`,`updated_at<$1 AND (cooldown_until IS NULL OR cooldown_until<=$2)`,[]any{now.Add(-pressureRetention),now},2000);err!=nil{return out,err}
	if err=tx.Commit(ctx);err!=nil{return out,err};return out,nil
}

func deleteBounded(ctx context.Context,tx pgx.Tx,table,key,predicate string,arg any,limit int64)(int64,error){
	args:=[]any{arg};if values,ok:=arg.([]any);ok{args=values}
	query:=`WITH doomed AS (SELECT `+key+` FROM `+table+` WHERE `+predicate+` LIMIT `+int64String(limit)+`) DELETE FROM `+table+` t USING doomed d WHERE t.`+key+`=d.`+key
	tag,err:=tx.Exec(ctx,query,args...);if err!=nil{return 0,err};return tag.RowsAffected(),nil
}

func int64String(v int64)string{
	if v<=0{return "1"}
	const digits="0123456789";buf:=[20]byte{};i:=len(buf)
	for v>0{i--;buf[i]=digits[v%10];v/=10}
	return string(buf[i:])
}
