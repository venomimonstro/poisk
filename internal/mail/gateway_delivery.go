package mail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type OutboundCallback struct {
	SourceEventID string
	DeliveryID int64
	Type string
	RemoteQueueID string
	Code string
	Detail string
}

func (r Repository) MarkOutboundSubmitted(ctx context.Context,deliveryID int64,worker,remoteQueueID string,now time.Time)error{
	if r.DB==nil||deliveryID<=0||strings.TrimSpace(worker)==""{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};remoteQueueID=strings.TrimSpace(remoteQueueID);if remoteQueueID==""||len(remoteQueueID)>255{return ErrInvalid}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var attempt int
	err=tx.QueryRow(ctx,`SELECT attempts FROM mail_outbound_deliveries WHERE delivery_id=$1 AND status='LEASED' AND lease_owner=$2 FOR UPDATE`,deliveryID,worker).Scan(&attempt);if errors.Is(err,pgx.ErrNoRows){return ErrConflict};if err!=nil{return err}
	if _,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='SUBMITTED',lease_owner=NULL,lease_until=NULL,remote_queue_id=$3,submitted_at=$4,updated_at=$4 WHERE delivery_id=$1 AND lease_owner=$2`,deliveryID,worker,remoteQueueID,now);err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'SUBMIT',$2,jsonb_build_object('remote_queue_id',$3))`,deliveryID,attempt,remoteQueueID);err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_gateway_events(direction,event_type,delivery_id,code,details) VALUES('OUTBOUND','ACCEPT',$1,NULL,jsonb_build_object('remote_queue_id',$2))`,deliveryID,remoteQueueID);err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) ApplyOutboundCallback(ctx context.Context,in OutboundCallback,now time.Time)(duplicate bool,err error){
	if r.DB==nil||in.DeliveryID<=0{return false,ErrInvalid};if now.IsZero(){now=time.Now().UTC()};in.SourceEventID=strings.TrimSpace(in.SourceEventID);in.Type=strings.ToUpper(strings.TrimSpace(in.Type));in.RemoteQueueID=strings.TrimSpace(in.RemoteQueueID);in.Code=strings.TrimSpace(in.Code);in.Detail=strings.TrimSpace(in.Detail)
	if len(in.SourceEventID)<16||len(in.SourceEventID)>160||strings.ContainsAny(in.SourceEventID,"\r\n\x00")||len(in.RemoteQueueID)>255||len(in.Code)>80||len(in.Detail)>1000{return false,ErrInvalid};if in.Type!="DELIVER"&&in.Type!="BOUNCE"{return false,ErrInvalid}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return false,err};defer func(){_=tx.Rollback(ctx)}()
	// Serialize the durable MTA event id before checking whether it was already applied.
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtext($1))`,in.SourceEventID);err!=nil{return false,err}
	var existingDelivery *int64;var existingType string
	err=tx.QueryRow(ctx,`SELECT delivery_id,event_type FROM mail_gateway_events WHERE source_event_id=$1`,in.SourceEventID).Scan(&existingDelivery,&existingType)
	if err==nil{if existingDelivery==nil||*existingDelivery!=in.DeliveryID||existingType!=in.Type{return false,ErrGatewayMismatch};return true,tx.Commit(ctx)};if !errors.Is(err,pgx.ErrNoRows){return false,err}
	var status string;var currentRemote *string;var messageID,mailboxID int64;var attempt int
	err=tx.QueryRow(ctx,`SELECT d.status,d.remote_queue_id,d.message_id,d.sender_mailbox_id,d.attempts FROM mail_outbound_deliveries d WHERE d.delivery_id=$1 FOR UPDATE`,in.DeliveryID).Scan(&status,&currentRemote,&messageID,&mailboxID,&attempt);if errors.Is(err,pgx.ErrNoRows){return false,ErrNotFound};if err!=nil{return false,err}
	if currentRemote!=nil&&in.RemoteQueueID!=""&&*currentRemote!=in.RemoteQueueID{return false,ErrGatewayMismatch}
	switch in.Type{
	case "DELIVER":
		if status=="DELIVERED"{break};if status!="SUBMITTED"{return false,ErrConflict}
		if _,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='DELIVERED',delivered_at=$2,updated_at=$2,last_error_code=NULL,last_error_detail=NULL WHERE delivery_id=$1`,in.DeliveryID,now);err!=nil{return false,err}
		if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'DELIVER',$2,jsonb_build_object('remote_queue_id',NULLIF($3,'')))`,in.DeliveryID,attempt,in.RemoteQueueID);err!=nil{return false,err}
	case "BOUNCE":
		if status=="BOUNCED"{break};if status!="SUBMITTED"&&status!="DELIVERED"{return false,ErrConflict}
		if _,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='BOUNCED',bounced_at=$2,updated_at=$2,last_error_code=NULLIF($3,''),last_error_detail=NULLIF($4,''), WHERE delivery_id=$1`,in.DeliveryID,now,in.Code,in.Detail);err!=nil{return false,err}
		if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'BOUNCE',$2,jsonb_build_object('remote_queue_id',NULLIF($3,''),'code',NULLIF($4,'')))`,in.DeliveryID,attempt,in.RemoteQueueID,in.Code);err!=nil{return false,err}
	}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_gateway_events(direction,event_type,message_id,delivery_id,mailbox_id,code,source_event_id,details) VALUES('OUTBOUND',$1,$2,$3,$4,NULLIF($5,''),$6,jsonb_build_object('remote_queue_id',NULLIF($7,''),'detail',NULLIF($8,'')))`,in.Type,messageID,in.DeliveryID,mailboxID,in.Code,in.SourceEventID,in.RemoteQueueID,in.Detail);err!=nil{return false,err}
	return false,tx.Commit(ctx)
}
