//go:build integration

package mail

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func mailDB(t *testing.T)*pgxpool.Pool{t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)};t.Cleanup(pool.Close);return pool}
func mailUser(t *testing.T,pool *pgxpool.Pool,prefix string)int64{t.Helper();var id int64;email:=fmt.Sprintf("mail-%s-%d@example.test",prefix,time.Now().UnixNano());if err:=pool.QueryRow(context.Background(),`INSERT INTO consumer_users(email,password_hash,email_verified_at) VALUES($1,'integration-password-hash-000000',now()) RETURNING user_id`,email).Scan(&id);err!=nil{t.Fatal(err)};return id}

func TestInternalSendIsAtomicAndTenantScoped(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();sender:=mailUser(t,pool,"sender");recipient:=mailUser(t,pool,"recipient");outsider:=mailUser(t,pool,"outsider")
	senderBox,err:=repo.EnsureMailbox(ctx,sender);if err!=nil{t.Fatal(err)};recipientBox,err:=repo.EnsureMailbox(ctx,recipient);if err!=nil{t.Fatal(err)};if _,err=repo.EnsureMailbox(ctx,outsider);err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Atomic delivery",BodyText:"Internal message body for atomic delivery test.",Recipients:[]Recipient{{Address:recipientBox.Address,Type:"TO"}}});if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	sent,err:=repo.ListFolder(ctx,sender,"SENT",20,0);if err!=nil{t.Fatal(err)};inbox,err:=repo.ListFolder(ctx,recipient,"INBOX",20,0);if err!=nil{t.Fatal(err)};if len(sent)!=1||len(inbox)!=1||sent[0].Message.ID!=draft.ID||inbox[0].Message.ID!=draft.ID{t.Fatalf("sent=%+v inbox=%+v",sent,inbox)}
	if _,err=repo.GetItem(ctx,outsider,inbox[0].ItemID);err!=ErrNotFound{t.Fatalf("outsider read err=%v",err)}
	var senderCopies,recipientCopies int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE mailbox_id=$1 AND message_id=$2`,senderBox.ID,draft.ID).Scan(&senderCopies);err!=nil{t.Fatal(err)};if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE mailbox_id=$1 AND message_id=$2`,recipientBox.ID,draft.ID).Scan(&recipientCopies);err!=nil{t.Fatal(err)};if senderCopies!=1||recipientCopies!=1{t.Fatalf("sender copies=%d recipient copies=%d",senderCopies,recipientCopies)}
}

func TestInvalidRecipientCannotPartiallySend(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();sender:=mailUser(t,pool,"atomic-invalid");valid:=mailUser(t,pool,"atomic-valid");validBox,err:=repo.EnsureMailbox(ctx,valid);if err!=nil{t.Fatal(err)};senderBox,err:=repo.EnsureMailbox(ctx,sender);if err!=nil{t.Fatal(err)}
	_,err=repo.CreateDraft(ctx,sender,DraftInput{Subject:"No partial delivery",BodyText:"This draft contains a missing internal recipient.",Recipients:[]Recipient{{Address:validBox.Address,Type:"TO"},{Address:"missing@internal.poisk",Type:"CC"}}});if err!=ErrNotFound{t.Fatalf("create draft should fail before persistence, got %v",err)}
	var messages,items int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_messages WHERE sender_mailbox_id=$1`,senderBox.ID).Scan(&messages);err!=nil{t.Fatal(err)};if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE mailbox_id=$1`,validBox.ID).Scan(&items);err!=nil{t.Fatal(err)};if messages!=0||items!=0{t.Fatalf("partial state messages=%d recipient items=%d",messages,items)}
}

func TestRecipientDeactivationCancelsEntireSend(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();sender:=mailUser(t,pool,"deactivate-sender");first:=mailUser(t,pool,"deactivate-first");second:=mailUser(t,pool,"deactivate-second")
	senderBox,err:=repo.EnsureMailbox(ctx,sender);if err!=nil{t.Fatal(err)};firstBox,err:=repo.EnsureMailbox(ctx,first);if err!=nil{t.Fatal(err)};secondBox,err:=repo.EnsureMailbox(ctx,second);if err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Recipient disabled before send",BodyText:"Delivery must be all or none if a recipient is disabled.",Recipients:[]Recipient{{Address:firstBox.Address,Type:"TO"},{Address:secondBox.Address,Type:"CC"}}});if err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`UPDATE mailboxes SET status='DISABLED',updated_at=now() WHERE mailbox_id=$1`,secondBox.ID);err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=ErrConflict{t.Fatalf("expected conflict, got %v",err)}
	var state string;if err=pool.QueryRow(ctx,`SELECT state FROM mail_messages WHERE message_id=$1`,draft.ID).Scan(&state);err!=nil{t.Fatal(err)};if state!="DRAFT"{t.Fatalf("message state=%s",state)}
	var senderDraft,firstDeliveries,secondDeliveries int
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE mailbox_id=$1 AND message_id=$2 AND item_role='DRAFT'`,senderBox.ID,draft.ID).Scan(&senderDraft);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE mailbox_id=$1 AND message_id=$2 AND item_role='DELIVERY'`,firstBox.ID,draft.ID).Scan(&firstDeliveries);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE mailbox_id=$1 AND message_id=$2 AND item_role='DELIVERY'`,secondBox.ID,draft.ID).Scan(&secondDeliveries);err!=nil{t.Fatal(err)}
	if senderDraft!=1||firstDeliveries!=0||secondDeliveries!=0{t.Fatalf("draft=%d first=%d second=%d",senderDraft,firstDeliveries,secondDeliveries)}
}

func TestSelfSendCreatesSentAndInboxCopies(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();user:=mailUser(t,pool,"self");box,err:=repo.EnsureMailbox(ctx,user);if err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,user,DraftInput{Subject:"Self delivery",BodyText:"Self delivery must create separate sent and inbox items.",Recipients:[]Recipient{{Address:box.Address,Type:"TO"},{Address:box.Address,Type:"CC"}}});if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,user,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	var sentCopies,deliveryCopies int;if err=pool.QueryRow(ctx,`SELECT count(*) FILTER(WHERE item_role='SENT_COPY'),count(*) FILTER(WHERE item_role='DELIVERY') FROM mail_items WHERE mailbox_id=$1 AND message_id=$2`,box.ID,draft.ID).Scan(&sentCopies,&deliveryCopies);err!=nil{t.Fatal(err)};if sentCopies!=1||deliveryCopies!=1{t.Fatalf("sent=%d delivery=%d",sentCopies,deliveryCopies)}
	var recipients int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_recipients WHERE message_id=$1`,draft.ID).Scan(&recipients);err!=nil{t.Fatal(err)};if recipients!=1{t.Fatalf("deduplicated recipients=%d",recipients)}
}

func TestTrashRestoreAndSearchStayInsideMailbox(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();sender:=mailUser(t,pool,"search-sender");recipient:=mailUser(t,pool,"search-recipient");box,err:=repo.EnsureMailbox(ctx,recipient);if err!=nil{t.Fatal(err)};if _,err=repo.EnsureMailbox(ctx,sender);err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Unique platypus subject",BodyText:"Searchable platypus message body.",Recipients:[]Recipient{{Address:box.Address,Type:"TO"}}});if err!=nil{t.Fatal(err)};if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	inbox,err:=repo.ListFolder(ctx,recipient,"INBOX",10,0);if err!=nil||len(inbox)!=1{t.Fatalf("inbox=%+v err=%v",inbox,err)};itemID:=inbox[0].ItemID
	found,err:=repo.Search(ctx,recipient,"platypus",10);if err!=nil||len(found)!=1{t.Fatalf("search=%+v err=%v",found,err)}
	if err=repo.MoveToTrash(ctx,recipient,itemID);err!=nil{t.Fatal(err)};found,err=repo.Search(ctx,recipient,"platypus",10);if err!=nil{t.Fatal(err)};if len(found)!=0{t.Fatalf("trash leaked into search: %+v",found)}
	if err=repo.Restore(ctx,recipient,itemID);err!=nil{t.Fatal(err)};inbox,err=repo.ListFolder(ctx,recipient,"INBOX",10,0);if err!=nil||len(inbox)!=1{t.Fatalf("restored inbox=%+v err=%v",inbox,err)}
}
