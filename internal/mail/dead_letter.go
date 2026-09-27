package mail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const MaxManualDeadRetries = 3

func RetryableDeadCode(code string) bool {
	code=strings.ToUpper(strings.TrimSpace(code))
	switch code {
	case "SUBMITTED_STALE","MTA_UNAVAILABLE","MTA_RESPONSE","MTA_ERROR":
		return true
	case "MTA_HTTP_429":
		return true
	}
	return strings.HasPrefix(code,"MTA_HTTP_5") && len(code)==12
}

type DeadLetter struct {
	DeliveryID int64 `json:"delivery_id"`
	MessageID int64 `json:"message_id"`
	SenderMailboxID int64 `json:"sender_mailbox_id"`
	Attempts int `json:"attempts"`
	ErrorCode string `json:"error_code"`
	Retryable bool `json:"retryable"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r Repository) ListDeadLetters(ctx context.Context,limit int,beforeID int64)([]DeadLetter,error){
	if r.DB==nil{return nil,ErrInvalid};if limit<=0{limit=50};if limit>200{limit=200};if beforeID<0{return nil,ErrInvalid}
	rows,err:=r.DB.Query(ctx,`SELECT delivery_id,message_id,sender_mailbox_id,attempts,COALESCE(last_error_code,''),updated_at
FROM mail_outbound_deliveries WHERE status='DEAD' AND ($1=0 OR delivery_id<$1) ORDER BY delivery_id DESC LIMIT $2`,beforeID,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]DeadLetter,0,limit);for rows.Next(){var x DeadLetter;if err=rows.Scan(&x.DeliveryID,&x.MessageID,&x.SenderMailboxID,&x.Attempts,&x.ErrorCode,&x.UpdatedAt);err!=nil{return nil,err};x.Retryable=RetryableDeadCode(x.ErrorCode);out=append(out,x)};return out,rows.Err()
}

func (r Repository) RetryDeadLetter(ctx context.Context,deliveryID int64,actor string,now time.Time)error{
	actor=strings.TrimSpace(actor);if r.DB==nil||deliveryID<=0||actor==""||len(actor)>120||strings.ContainsAny(actor,"\r\n\x00"){return ErrInvalid};if now.IsZero(){now=time.Now().UTC()}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var status,code,address string;var senderMailboxID int64;var previousAttempts int
	err=tx.QueryRow(ctx,`SELECT d.status,COALESCE(d.last_error_code,''),d.sender_mailbox_id,d.attempts,r.address
FROM mail_outbound_deliveries d JOIN mail_external_recipients r ON r.external_recipient_id=d.external_recipient_id
WHERE d.delivery_id=$1 FOR UPDATE OF d`,deliveryID).Scan(&status,&code,&senderMailboxID,&previousAttempts,&address)
	if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err};if status!="DEAD"{return ErrConflict};if !RetryableDeadCode(code){return ErrForbidden}
	suppressed,err:=isDeliverySuppressedTx(ctx,tx,senderMailboxID,address,now);if err!=nil{return err};if suppressed{return ErrForbidden}
	var manualRetries int;if err=tx.QueryRow(ctx,`SELECT count(*) FROM mail_outbound_events WHERE delivery_id=$1 AND action='DEAD_RETRY'`,deliveryID).Scan(&manualRetries);err!=nil{return err};if manualRetries>=MaxManualDeadRetries{return ErrRateLimited}
	tag,err:=tx.Exec(ctx,`UPDATE mail_outbound_deliveries SET status='RETRY',attempts=0,next_attempt_at=$2,lease_owner=NULL,lease_until=NULL,last_error_code=NULL,last_error_detail=NULL,submitted_at=NULL,bounced_at=NULL,updated_at=$2 WHERE delivery_id=$1 AND status='DEAD'`,deliveryID,now);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrConflict}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_outbound_events(delivery_id,action,attempt,details) VALUES($1,'DEAD_RETRY',$2,jsonb_build_object('previous_code',$3,'manual_retry',$4,'actor',$5))`,deliveryID,previousAttempts,code,manualRetries+1,actor);err!=nil{return err}
	return tx.Commit(ctx)
}
