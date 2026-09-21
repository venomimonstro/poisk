package billing

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type PaymentEvent struct {
	Provider string
	ProviderEventID string
	Type string
	InvoiceID int64
	AmountKopecks int64
	Currency string
	Payload []byte
	OccurredAt time.Time
}

type PaymentResult struct{EventID int64 `json:"payment_event_id"`;Processed bool `json:"processed"`;InvoiceStatus string `json:"invoice_status"`;SubscriptionStatus string `json:"subscription_status"`}

func (r *Repository) ApplyPaymentEvent(ctx context.Context,in PaymentEvent)(PaymentResult,error){
	in.Provider=strings.ToUpper(strings.TrimSpace(in.Provider));in.ProviderEventID=strings.TrimSpace(in.ProviderEventID);in.Type=strings.ToUpper(strings.TrimSpace(in.Type));in.Currency=strings.ToUpper(strings.TrimSpace(in.Currency));if in.OccurredAt.IsZero(){in.OccurredAt=time.Now().UTC()}
	if r==nil||r.db==nil||len(in.Provider)<2||len(in.Provider)>32||len(in.ProviderEventID)<1||len(in.ProviderEventID)>160||in.InvoiceID<=0||in.AmountKopecks<0||in.Currency!="RUB"||!validPaymentType(in.Type){return PaymentResult{},ErrInvalid}
	hash:=hashPayload(in.Payload);tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return PaymentResult{},err};defer func(){_=tx.Rollback(ctx)}()
	var eventID int64;var storedHash []byte;var processed *time.Time
	err=tx.QueryRow(ctx,`INSERT INTO billing_payment_events(provider,provider_event_id,event_type,invoice_id,amount_kopecks,currency,payload_hash,occurred_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(provider,provider_event_id) DO UPDATE SET provider_event_id=EXCLUDED.provider_event_id
RETURNING payment_event_id,payload_hash,processed_at`,in.Provider,in.ProviderEventID,in.Type,in.InvoiceID,in.AmountKopecks,in.Currency,hash[:],in.OccurredAt).Scan(&eventID,&storedHash,&processed);if err!=nil{return PaymentResult{},err};if !bytes.Equal(storedHash,hash[:]){return PaymentResult{},ErrPaymentMismatch}
	var invoiceStatus,currency string;var amount,accountID,subscriptionID int64
	err=tx.QueryRow(ctx,`SELECT status,currency,amount_kopecks,account_id,subscription_id FROM billing_invoices WHERE invoice_id=$1 FOR UPDATE`,in.InvoiceID).Scan(&invoiceStatus,&currency,&amount,&accountID,&subscriptionID);if errors.Is(err,pgx.ErrNoRows){return PaymentResult{},ErrNotFound};if err!=nil{return PaymentResult{},err};if currency!=in.Currency{return PaymentResult{},ErrPaymentMismatch}
	var subscriptionStatus string;if err=tx.QueryRow(ctx,`SELECT status FROM billing_subscriptions WHERE subscription_id=$1 FOR UPDATE`,subscriptionID).Scan(&subscriptionStatus);err!=nil{return PaymentResult{},err}
	if processed!=nil{if err=tx.Commit(ctx);err!=nil{return PaymentResult{},err};return PaymentResult{EventID:eventID,Processed:false,InvoiceStatus:invoiceStatus,SubscriptionStatus:subscriptionStatus},nil}

	switch in.Type{
	case "PAYMENT_SUCCEEDED":
		if in.AmountKopecks!=amount{return PaymentResult{},ErrPaymentMismatch};if invoiceStatus=="VOID"||invoiceStatus=="REFUNDED"{return PaymentResult{},ErrConflict}
		if _,err=tx.Exec(ctx,`UPDATE billing_invoices SET status='PAID',paid_at=COALESCE(paid_at,now()),updated_at=now() WHERE invoice_id=$1`,in.InvoiceID);err!=nil{return PaymentResult{},err}
		if _,err=tx.Exec(ctx,`UPDATE billing_subscriptions SET status='ACTIVE',activated_at=COALESCE(activated_at,now()),grace_until=current_period_end+interval '3 days',updated_at=now() WHERE subscription_id=$1`,subscriptionID);err!=nil{return PaymentResult{},err}
		if _,err=tx.Exec(ctx,`INSERT INTO billing_ledger_entries(account_id,invoice_id,payment_event_id,entry_type,amount_kopecks,currency,idempotency_key,details) VALUES($1,$2,$3,'PAYMENT',$4,$5,$6,'{}') ON CONFLICT(idempotency_key) DO NOTHING`,accountID,in.InvoiceID,eventID,-in.AmountKopecks,in.Currency,"payment-event:"+in.Provider+":"+in.ProviderEventID);err!=nil{return PaymentResult{},err};invoiceStatus="PAID";subscriptionStatus="ACTIVE"
	case "PAYMENT_FAILED":
		if invoiceStatus=="OPEN"{if _,err=tx.Exec(ctx,`UPDATE billing_invoices SET status='FAILED',updated_at=now() WHERE invoice_id=$1`,in.InvoiceID);err!=nil{return PaymentResult{},err};invoiceStatus="FAILED"}
		if subscriptionStatus=="PENDING"{if _,err=tx.Exec(ctx,`UPDATE billing_subscriptions SET status='PAST_DUE',updated_at=now() WHERE subscription_id=$1`,subscriptionID);err!=nil{return PaymentResult{},err};subscriptionStatus="PAST_DUE"}
	case "REFUND_SUCCEEDED","CHARGEBACK":
		if invoiceStatus!="PAID"&&invoiceStatus!="REFUNDED"{return PaymentResult{},ErrConflict};if in.AmountKopecks<=0{return PaymentResult{},ErrPaymentMismatch}
		var already int64;if err=tx.QueryRow(ctx,`SELECT COALESCE(sum(amount_kopecks),0) FROM billing_ledger_entries WHERE invoice_id=$1 AND entry_type='REFUND'`,in.InvoiceID).Scan(&already);err!=nil{return PaymentResult{},err};if already+in.AmountKopecks>amount{return PaymentResult{},ErrPaymentMismatch}
		if _,err=tx.Exec(ctx,`INSERT INTO billing_ledger_entries(account_id,invoice_id,payment_event_id,entry_type,amount_kopecks,currency,idempotency_key,details) VALUES($1,$2,$3,'REFUND',$4,$5,$6,jsonb_build_object('reason',$7)) ON CONFLICT(idempotency_key) DO NOTHING`,accountID,in.InvoiceID,eventID,in.AmountKopecks,in.Currency,"payment-event:"+in.Provider+":"+in.ProviderEventID,in.Type);err!=nil{return PaymentResult{},err}
		if already+in.AmountKopecks==amount{if _,err=tx.Exec(ctx,`UPDATE billing_invoices SET status='REFUNDED',updated_at=now() WHERE invoice_id=$1`,in.InvoiceID);err!=nil{return PaymentResult{},err};if _,err=tx.Exec(ctx,`UPDATE billing_subscriptions SET status='CANCELED',canceled_at=COALESCE(canceled_at,now()),updated_at=now() WHERE subscription_id=$1`,subscriptionID);err!=nil{return PaymentResult{},err};invoiceStatus="REFUNDED";subscriptionStatus="CANCELED"}
	}
	if _,err=tx.Exec(ctx,`UPDATE billing_payment_events SET processed_at=now() WHERE payment_event_id=$1 AND processed_at IS NULL`,eventID);err!=nil{return PaymentResult{},err};if err=tx.Commit(ctx);err!=nil{return PaymentResult{},err};return PaymentResult{EventID:eventID,Processed:true,InvoiceStatus:invoiceStatus,SubscriptionStatus:subscriptionStatus},nil
}

func validPaymentType(v string)bool{switch v{case "PAYMENT_SUCCEEDED","PAYMENT_FAILED","REFUND_SUCCEEDED","CHARGEBACK":return true};return false}

func (r *Repository) ReconcileLifecycle(ctx context.Context,now time.Time)(graced,expired int64,err error){
	if r==nil||r.db==nil{return 0,0,ErrInvalid};if now.IsZero(){now=time.Now().UTC()};tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,0,err};defer func(){_=tx.Rollback(ctx)}()
	tag,err:=tx.Exec(ctx,`UPDATE billing_subscriptions SET status='GRACE',grace_until=COALESCE(grace_until,current_period_end+interval '3 days'),updated_at=now() WHERE status='ACTIVE' AND current_period_end<=$1`,now);if err!=nil{return 0,0,err};graced=tag.RowsAffected()
	tag,err=tx.Exec(ctx,`UPDATE billing_subscriptions SET status='EXPIRED',updated_at=now() WHERE status IN ('GRACE','PAST_DUE') AND COALESCE(grace_until,current_period_end)<=$1`,now);if err!=nil{return 0,0,err};expired=tag.RowsAffected();if err=tx.Commit(ctx);err!=nil{return 0,0,err};return graced,expired,nil
}
