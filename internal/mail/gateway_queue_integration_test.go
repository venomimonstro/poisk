//go:build integration

package mail

import (
	"context"
	"errors"
	"testing"
	"time"
)

func seedOutboundDelivery(t *testing.T,repo Repository) OutboundDelivery {
	t.Helper();ctx:=context.Background();sender:=mailUser(t,repo.DB,"queue-sender");box,err:=repo.EnsureMailbox(ctx,sender);if err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Queue delivery",BodyText:"Durable outbound queue integration test message.",Recipients:[]Recipient{{Address:"queue-target@example.net",Type:"TO"}}});if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	var d OutboundDelivery
	if err=repo.DB.QueryRow(ctx,`SELECT d.delivery_id,d.message_id,d.external_recipient_id,d.sender_mailbox_id,r.address,r.recipient_type,d.status,d.attempts,d.idempotency_key FROM mail_outbound_deliveries d JOIN mail_external_recipients r ON r.external_recipient_id=d.external_recipient_id WHERE d.message_id=$1`,draft.ID).Scan(&d.ID,&d.MessageID,&d.ExternalRecipientID,&d.SenderMailboxID,&d.Address,&d.RecipientType,&d.Status,&d.Attempts,&d.IdempotencyKey);err!=nil{t.Fatal(err)}
	if d.SenderMailboxID!=box.ID{t.Fatalf("sender mailbox=%d want=%d",d.SenderMailboxID,box.ID)}
	return d
}

func TestOutboundLeaseIsExclusiveAndWorkerOwned(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();seed:=seedOutboundDelivery(t,repo);now:=time.Now().UTC()
	first,err:=repo.LeaseOutbound(ctx,"worker-a",10,30*time.Second,now);if err!=nil{t.Fatal(err)};if len(first)!=1||first[0].ID!=seed.ID{t.Fatalf("first lease=%+v",first)}
	second,err:=repo.LeaseOutbound(ctx,"worker-b",10,30*time.Second,now);if err!=nil{t.Fatal(err)};if len(second)!=0{t.Fatalf("second worker double leased: %+v",second)}
	if err=repo.MarkOutboundDelivered(ctx,seed.ID,"worker-b","remote-1",now);!errors.Is(err,ErrConflict){t.Fatalf("foreign ack err=%v",err)}
	if err=repo.MarkOutboundDelivered(ctx,seed.ID,"worker-a","remote-1",now);err!=nil{t.Fatal(err)}
	var status,remote string;if err=pool.QueryRow(ctx,`SELECT status,COALESCE(remote_queue_id,'') FROM mail_outbound_deliveries WHERE delivery_id=$1`,seed.ID).Scan(&status,&remote);err!=nil{t.Fatal(err)};if status!="DELIVERED"||remote!="remote-1"{t.Fatalf("status=%s remote=%s",status,remote)}
}

func TestExpiredLeaseRecoversAndRetriesToDead(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();seed:=seedOutboundDelivery(t,repo);base:=time.Now().UTC()
	leased,err:=repo.LeaseOutbound(ctx,"worker-a",1,time.Second,base);if err!=nil||len(leased)!=1{t.Fatalf("lease=%+v err=%v",leased,err)}
	if err=repo.RecoverExpiredOutboundLeases(ctx,base.Add(2*time.Second));err!=nil{t.Fatal(err)}
	var status string;if err=pool.QueryRow(ctx,`SELECT status FROM mail_outbound_deliveries WHERE delivery_id=$1`,seed.ID).Scan(&status);err!=nil{t.Fatal(err)};if status!="RETRY"{t.Fatalf("recovered status=%s",status)}

	now:=base.Add(3*time.Second)
	for attempt:=2;attempt<=8;attempt++{
		if _,err=pool.Exec(ctx,`UPDATE mail_outbound_deliveries SET next_attempt_at=$2 WHERE delivery_id=$1`,seed.ID,now);err!=nil{t.Fatal(err)}
		items,leaseErr:=repo.LeaseOutbound(ctx,"worker-a",1,time.Minute,now);if leaseErr!=nil||len(items)!=1{t.Fatalf("attempt=%d lease=%+v err=%v",attempt,items,leaseErr)}
		if err=repo.RetryOutbound(ctx,seed.ID,"worker-a","TEMP","temporary failure",now);err!=nil{t.Fatal(err)}
		if err=pool.QueryRow(ctx,`SELECT status FROM mail_outbound_deliveries WHERE delivery_id=$1`,seed.ID).Scan(&status);err!=nil{t.Fatal(err)}
		if attempt<8&&status!="RETRY"{t.Fatalf("attempt=%d status=%s",attempt,status)}
		if attempt==8&&status!="DEAD"{t.Fatalf("attempt=%d status=%s",attempt,status)}
		now=now.Add(time.Hour)
	}
}
