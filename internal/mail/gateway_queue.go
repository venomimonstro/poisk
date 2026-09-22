package mail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type OutboundDelivery struct {
	ID int64 `json:"delivery_id"`
	MessageID int64 `json:"message_id"`
	ExternalRecipientID int64 `json:"external_recipient_id"`
	SenderMailboxID int64 `json:"sender_mailbox_id"`
	Address string `json:"address"`
	RecipientType string `json:"recipient_type"`
	Status string `json:"status"`
	Attempts int `json:"attempts"`
	IdempotencyKey string `json:"idempotency_key"`
	LeaseOwner string `json:"-"`
	LeaseUntil *time.Time `json:"lease_until,omitempty"`
}

func NormalizeExternalAddress(raw string)(string,error){
	raw=strings.TrimSpace(raw);if len(raw)<3||len(raw)>320||strings.ContainsAny(raw,"\r\n\x00"){return "",ErrInvalid}
	parsed,err:=mail.ParseAddress(raw);if err!=nil||parsed.Address!=raw{return "",ErrInvalid}
	at:=strings.LastIndexByte(raw,'@');if at<=0||at==len(raw)-1{return "",ErrInvalid}
	local:=raw[:at];domain:=strings.ToLower(strings.TrimSuffix(raw[at+1:],"."));if domain==""||domain=="internal.poisk"||len(local)>64||len(domain)>253{return "",ErrInvalid}
	if strings.ContainsAny(local," <>(),:;\\\"[]") {return "",ErrInvalid}
	return local+"@"+domain,nil
}

func outboundIdempotency(messageID,recipientID int64,address string)string{sum:=sha256.Sum256([]byte(fmt.Sprintf("mail-outbound:%d:%d:%s",messageID,recipientID,strings.ToLower(address))));return hex.EncodeToString(sum[:])}

func (r Repository) LeaseOutbound(ctx context.Context,worker string,limit int,lease time.Duration,now time.Time)([]OutboundDelivery,error){
	if r.DB==nil||strings.TrimSpace(worker)==""||lease<=0{return nil,ErrInvalid};if limit<=0{limit=20};if limit>100{limit=100};if now.IsZero(){now=time.Now().UTC()}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return nil,err};defer func(){_=tx.Rollback(ctx)}()
	rows,err:=tx.Query(ctx,`WITH picked AS (
 SELECT d.delivery_id FROM mail_outbound_deliveries d
 WHERE d.status IN ('READY','RETRY') AND d.next_attempt_at<=$1
 ORDER BY d.next_attempt_at,d.delivery_id FOR UPDATE SKIP LOCKED LIMIT $2
)
UPDATE mail_outbound_deliveries d SET status='LEASED',lease_owner=$3,lease_until=$1+$4::interval,attempts=d.attempts+1,updated_at=$1
FROM picked p WHERE d.delivery_id=p.delivery_id
RETURNING d.delivery_id,d.message_id,d.external_recipient_id,d.sender_mailbox_id,d.status,d.attempts,d.idempotency_key,d.lease_owner,d.lease_until`,now,limit,worker,lease.String());if err!=nil{return nil,err};defer rows.Close()
	out:=[]OutboundDelivery{};for rows.Next(){var d OutboundDelivery;if err=rows.Scan(&d.ID,&d.MessageID,&d.ExternalRecipientID,&d.SenderMailboxID,&d.Status,&d.Attempts,&d.IdempotencyKey,&d.LeaseOwner,&d.LeaseUntil);err!=nil{return nil,err};if err=tx.QueryRow(ctx,`SELECT address,recipient_type FROM mail_external_recipients WHERE external_recipient_id=$1`,d.ExternalRecipientID).Scan(&d.Address,&d.RecipientType);err!=nil{return nil,err};out=append(out,d)};if err=rows.Err();err!=nil{return nil,err}
	for _,d:=range out{if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'LEASE',$2,jsonb_build_object('worker',$3))`,d.ID,d.Attempts,worker);err!=nil{return nil,err}}
	if err=tx.Commit(ctx);err!=nil{return nil,err};return out,nil
}

func (r Repository) RecoverExpiredOutboundLeases(ctx context.Context,now time.Time)error{
	if r.DB==nil{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	rows,err:=tx.Query(ctx,`UPDATE mail_outbound_deliveries SET status='RETRY',lease_owner=NULL,lease_until=NULL,next_attempt_at=$1,updated_at=$1 WHERE status='LEASED' AND lease_until<$1 RETURNING delivery_id,attempts`,now);if err!=nil{return err};type row struct{id int64;attempt int};items:=[]row{};for rows.Next(){var x row;if err=rows.Scan(&x.id,&x.attempt);err!=nil{rows.Close();return err};items=append(items,x)};if err=rows.Err();err!=nil{rows.Close();return err};rows.Close();for _,x:=range items{if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt) VALUES($1,'LEASE_EXPIRE',$2)`,x.id,x.attempt);err!=nil{return err}};return tx.Commit(ctx)
}

func (r Repository) MarkOutboundDelivered(ctx context.Context,deliveryID int64,worker,remoteQueueID string,now time.Time)error{return r.finishOutbound(ctx,deliveryID,worker,"DELIVERED","DELIVER",remoteQueueID,"","",now)}
func (r Repository) MarkOutboundBounced(ctx context.Context,deliveryID int64,worker,code,detail string,now time.Time)error{return r.finishOutbound(ctx,deliveryID,worker,"BOUNCED","BOUNCE","",code,detail,now)}

func (r Repository) RetryOutbound(ctx context.Context,deliveryID int64,worker,code,detail string,now time.Time)error{
	if r.DB==nil||deliveryID<=0||strings.TrimSpace(worker)==""{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};code=strings.TrimSpace(code);detail=strings.TrimSpace(detail);if len(code)>80||len(detail)>1000{return ErrInvalid}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var attempt int
	err=tx.QueryRow(ctx,`SELECT attempts FROM mail_outbound_deliveries WHERE delivery_id=$1 AND status='LEASED' AND lease_owner=$2 FOR UPDATE`,deliveryID,worker).Scan(&attempt);if errors.Is(err,pgx.ErrNoRows){return ErrConflict};if err!=nil{return err}
	status:="RETRY";action:="RETRY";delay:=time.Minute*time.Duration(1<<minInt(attempt-1,6));if attempt>=8{status="DEAD";action="DEAD";delay=0}
	_,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status=$3,lease_owner=NULL,lease_until=NULL,next_attempt_at=$4,last_error_code=NULLIF($5,''),last_error_detail=NULLIF($6,''),updated_at=$4 WHERE delivery_id=$1 AND lease_owner=$2`,deliveryID,worker,status,now.Add(delay),code,detail);if err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,$2,$3,jsonb_build_object('code',$4))`,deliveryID,action,attempt,code);err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) finishOutbound(ctx context.Context,deliveryID int64,worker,status,action,remoteQueueID,code,detail string,now time.Time)error{
	if r.DB==nil||deliveryID<=0||strings.TrimSpace(worker)==""{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};remoteQueueID=strings.TrimSpace(remoteQueueID);code=strings.TrimSpace(code);detail=strings.TrimSpace(detail);if len(remoteQueueID)>255||len(code)>80||len(detail)>1000{return ErrInvalid}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var attempt int
	err=tx.QueryRow(ctx,`SELECT attempts FROM mail_outbound_deliveries WHERE delivery_id=$1 AND status='LEASED' AND lease_owner=$2 FOR UPDATE`,deliveryID,worker).Scan(&attempt);if errors.Is(err,pgx.ErrNoRows){return ErrConflict};if err!=nil{return err}
	if status=="DELIVERED"{_,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='DELIVERED',lease_owner=NULL,lease_until=NULL,remote_queue_id=NULLIF($3,''),delivered_at=$4,updated_at=$4 WHERE delivery_id=$1 AND lease_owner=$2`,deliveryID,worker,remoteQueueID,now)}else{_,err=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='BOUNCED',lease_owner=NULL,lease_until=NULL,last_error_code=NULLIF($3,''),last_error_detail=NULLIF($4,''),bounced_at=$5,updated_at=$5 WHERE delivery_id=$1 AND lease_owner=$2`,deliveryID,worker,code,detail,now)};if err!=nil{return err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,$2,$3,jsonb_build_object('remote_queue_id',NULLIF($4,''),'code',NULLIF($5,'')))`,deliveryID,action,attempt,remoteQueueID,code);err!=nil{return err};return tx.Commit(ctx)
}

func minInt(a,b int)int{if a<b{return a};return b}
