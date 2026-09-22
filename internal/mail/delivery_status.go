package mail

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type DeliveryStatus struct {
	DeliveryID int64 `json:"delivery_id"`
	Address string `json:"address"`
	RecipientType string `json:"recipient_type"`
	Status string `json:"status"`
	Attempts int `json:"attempts"`
	RemoteQueueID string `json:"remote_queue_id,omitempty"`
	LastErrorCode string `json:"last_error_code,omitempty"`
	LastErrorDetail string `json:"last_error_detail,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	BouncedAt *time.Time `json:"bounced_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r Repository) OutboundStatuses(ctx context.Context,userID,messageID int64)([]DeliveryStatus,error){
	if r.DB==nil||userID<=0||messageID<=0{return nil,ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return nil,err}
	var owned bool;if err=r.DB.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_messages WHERE message_id=$1 AND sender_mailbox_id=$2 AND state='SENT')`,messageID,box.ID).Scan(&owned);err!=nil{return nil,err};if !owned{return nil,ErrNotFound}
	rows,err:=r.DB.Query(ctx,`SELECT d.delivery_id,e.address,e.recipient_type,d.status,d.attempts,COALESCE(d.remote_queue_id,''),COALESCE(d.last_error_code,''),COALESCE(d.last_error_detail,''),d.submitted_at,d.delivered_at,d.bounced_at,d.updated_at FROM mail_outbound_deliveries d JOIN mail_external_recipients e ON e.external_recipient_id=d.external_recipient_id WHERE d.message_id=$1 AND d.sender_mailbox_id=$2 ORDER BY e.recipient_type,e.ordinal,d.delivery_id`,messageID,box.ID);if err!=nil{return nil,err};defer rows.Close();out:=[]DeliveryStatus{};for rows.Next(){var x DeliveryStatus;if err=rows.Scan(&x.DeliveryID,&x.Address,&x.RecipientType,&x.Status,&x.Attempts,&x.RemoteQueueID,&x.LastErrorCode,&x.LastErrorDetail,&x.SubmittedAt,&x.DeliveredAt,&x.BouncedAt,&x.UpdatedAt);err!=nil{return nil,err};out=append(out,x)};if err=rows.Err();err!=nil{return nil,err};return out,nil
}

func (r Repository) OutboundStatus(ctx context.Context,userID,deliveryID int64)(DeliveryStatus,error){
	if r.DB==nil||userID<=0||deliveryID<=0{return DeliveryStatus{},ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return DeliveryStatus{},err};var x DeliveryStatus
	err=r.DB.QueryRow(ctx,`SELECT d.delivery_id,e.address,e.recipient_type,d.status,d.attempts,COALESCE(d.remote_queue_id,''),COALESCE(d.last_error_code,''),COALESCE(d.last_error_detail,''),d.submitted_at,d.delivered_at,d.bounced_at,d.updated_at FROM mail_outbound_deliveries d JOIN mail_external_recipients e ON e.external_recipient_id=d.external_recipient_id WHERE d.delivery_id=$1 AND d.sender_mailbox_id=$2`,deliveryID,box.ID).Scan(&x.DeliveryID,&x.Address,&x.RecipientType,&x.Status,&x.Attempts,&x.RemoteQueueID,&x.LastErrorCode,&x.LastErrorDetail,&x.SubmittedAt,&x.DeliveredAt,&x.BouncedAt,&x.UpdatedAt);if errors.Is(err,pgx.ErrNoRows){return DeliveryStatus{},ErrNotFound};return x,err
}
