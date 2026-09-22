//go:build integration

package mail

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMixedLocalExternalSendIsAtomicAndQueuesOutbound(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background()
	sender:=mailUser(t,pool,"mixed-sender");local:=mailUser(t,pool,"mixed-local")
	localBox,err:=repo.EnsureMailbox(ctx,local);if err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Mixed delivery",BodyText:"One local and two external recipients in one atomic send.",Recipients:[]Recipient{{Address:localBox.Address,Type:"TO"},{Address:"outside@example.net",Type:"CC"},{Address:"hidden@example.org",Type:"BCC"}}});if err!=nil{t.Fatal(err)}
	var localRows,externalRows int
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_recipients WHERE message_id=$1`,draft.ID).Scan(&localRows);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_external_recipients WHERE message_id=$1`,draft.ID).Scan(&externalRows);err!=nil{t.Fatal(err)}
	if localRows!=1||externalRows!=2{t.Fatalf("local=%d external=%d",localRows,externalRows)}

	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	inbox,err:=repo.ListFolder(ctx,local,"INBOX",10,0);if err!=nil||len(inbox)!=1{t.Fatalf("local inbox=%+v err=%v",inbox,err)}
	localItem,err:=repo.GetItem(ctx,local,inbox[0].ItemID);if err!=nil{t.Fatal(err)}
	for _,r:=range localItem.Message.Recipients{if r.Type=="BCC"{t.Fatalf("local recipient leaked bcc: %+v",localItem.Message.Recipients)}}
	seenCC:=false;for _,r:=range localItem.Message.Recipients{if r.Type=="CC"&&r.Address=="outside@example.net"{seenCC=true}};if !seenCC{t.Fatalf("local recipient missing external cc: %+v",localItem.Message.Recipients)}

	sent,err:=repo.ListFolder(ctx,sender,"SENT",10,0);if err!=nil||len(sent)!=1{t.Fatalf("sent=%+v err=%v",sent,err)}
	sentItem,err:=repo.GetItem(ctx,sender,sent[0].ItemID);if err!=nil{t.Fatal(err)}
	seenBCC:=false;for _,r:=range sentItem.Message.Recipients{if r.Type=="BCC"&&r.Address=="hidden@example.org"{seenBCC=true}};if !seenBCC{t.Fatalf("sender missing bcc: %+v",sentItem.Message.Recipients)}
	var ready int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_outbound_deliveries WHERE message_id=$1 AND status='READY'`,draft.ID).Scan(&ready);err!=nil{t.Fatal(err)};if ready!=2{t.Fatalf("ready outbound=%d",ready)}
	var queueEvents int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_outbound_events e JOIN mail_outbound_deliveries d ON d.delivery_id=e.delivery_id WHERE d.message_id=$1 AND e.action='QUEUE'`,draft.ID).Scan(&queueEvents);err!=nil{t.Fatal(err)};if queueEvents!=2{t.Fatalf("queue events=%d",queueEvents)}
}

func TestLocalExternalAliasIsDeliveredLocallyAndAddressPreserved(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background()
	sender:=mailUser(t,pool,"alias-sender");recipient:=mailUser(t,pool,"alias-recipient");box,err:=repo.EnsureMailbox(ctx,recipient);if err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`INSERT INTO mail_external_aliases(mailbox_id,local_part,domain,is_primary) VALUES($1,'support','poisk-mail.example',TRUE)`,box.ID);err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Local alias",BodyText:"Public-looking alias must resolve to the canonical local mailbox.",Recipients:[]Recipient{{Address:"support@poisk-mail.example",Type:"TO"}}});if err!=nil{t.Fatal(err)}
	var localRows,externalRows int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_recipients WHERE message_id=$1`,draft.ID).Scan(&localRows);err!=nil{t.Fatal(err)};if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_external_recipients WHERE message_id=$1`,draft.ID).Scan(&externalRows);err!=nil{t.Fatal(err)};if localRows!=1||externalRows!=0{t.Fatalf("local=%d external=%d",localRows,externalRows)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	inbox,err:=repo.ListFolder(ctx,recipient,"INBOX",10,0);if err!=nil||len(inbox)!=1{t.Fatalf("inbox=%+v err=%v",inbox,err)}
	sent,err:=repo.ListFolder(ctx,sender,"SENT",10,0);if err!=nil||len(sent)!=1{t.Fatalf("sent=%+v err=%v",sent,err)};item,err:=repo.GetItem(ctx,sender,sent[0].ItemID);if err!=nil{t.Fatal(err)};if len(item.Message.Recipients)!=1||item.Message.Recipients[0].Address!="support@poisk-mail.example"{t.Fatalf("delivery address lost: %+v",item.Message.Recipients)}
}

func TestUnknownAddressOnOwnedAliasDomainDoesNotRelayExternally(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();sender:=mailUser(t,pool,"owned-domain-sender");owner:=mailUser(t,pool,"owned-domain-owner");box,err:=repo.EnsureMailbox(ctx,owner);if err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`INSERT INTO mail_external_aliases(mailbox_id,local_part,domain,is_primary) VALUES($1,'known','owned-mail.example',TRUE)`,box.ID);err!=nil{t.Fatal(err)}
	_,err=repo.CreateDraft(ctx,sender,DraftInput{Subject:"No local domain relay",BodyText:"Unknown own-domain recipient must fail locally.",Recipients:[]Recipient{{Address:"missing@owned-mail.example",Type:"TO"}}})
	if !errors.Is(err,ErrNotFound){t.Fatalf("unknown own-domain address err=%v",err)}
	var externalRows int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_external_recipients e JOIN mail_messages m ON m.message_id=e.message_id JOIN mailboxes mb ON mb.mailbox_id=m.sender_mailbox_id WHERE mb.consumer_user_id=$1`,sender).Scan(&externalRows);err!=nil{t.Fatal(err)};if externalRows!=0{t.Fatalf("external fallback rows=%d",externalRows)}
}
