//go:build integration

package mail

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestTrashRetentionWaitsForAllCopiesAndFreesBlobQuota(t *testing.T){
	pool:=mailDB(t);ctx:=context.Background();repo:=Repository{DB:pool};store:=AttachmentStore{Repo:repo,Root:t.TempDir()}
	sender:=mailUser(t,pool,"retention-sender");recipient:=mailUser(t,pool,"retention-recipient")
	senderBox,err:=repo.EnsureMailbox(ctx,sender);if err!=nil{t.Fatal(err)}
	recipientBox,err:=repo.EnsureMailbox(ctx,recipient);if err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Retention message",BodyText:"Retention must preserve other mailbox copies until all expire.",Recipients:[]Recipient{{Address:recipientBox.Address,Type:"TO"}}});if err!=nil{t.Fatal(err)}
	payload:=[]byte("retention attachment")
	attachment,err:=store.Upload(ctx,sender,draft.ID,"retention.txt","text/plain",bytes.NewReader(payload),time.Now().UTC());if err!=nil{t.Fatal(err)}
	resolved,err:=store.ResolveDownload(ctx,sender,attachment.ID);if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	sent,err:=repo.ListFolder(ctx,sender,"SENT",10,0);if err!=nil||len(sent)!=1{t.Fatalf("sent=%+v err=%v",sent,err)}
	inbox,err:=repo.ListFolder(ctx,recipient,"INBOX",10,0);if err!=nil||len(inbox)!=1{t.Fatalf("inbox=%+v err=%v",inbox,err)}
	if err=repo.MoveToTrash(ctx,sender,sent[0].ItemID);err!=nil{t.Fatal(err)}
	old:=time.Now().UTC().Add(-31*24*time.Hour)
	if _,err=pool.Exec(ctx,`UPDATE mail_items SET deleted_at=$2 WHERE mail_item_id=$1`,sent[0].ItemID,old);err!=nil{t.Fatal(err)}
	if err=repo.PruneTrash(ctx,time.Now().UTC(),100);err!=nil{t.Fatal(err)}
	var messageCount,attachmentCount int
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_messages WHERE message_id=$1`,draft.ID).Scan(&messageCount);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_attachments WHERE attachment_id=$1`,attachment.ID).Scan(&attachmentCount);err!=nil{t.Fatal(err)}
	if messageCount!=1||attachmentCount!=1{t.Fatalf("sender-only expiry destroyed shared message: message=%d attachment=%d",messageCount,attachmentCount)}
	var used int64;if err=pool.QueryRow(ctx,`SELECT storage_used_bytes FROM mailboxes WHERE mailbox_id=$1`,senderBox.ID).Scan(&used);err!=nil{t.Fatal(err)};if used!=int64(len(payload)){t.Fatalf("quota released too early: %d",used)}

	if err=repo.MoveToTrash(ctx,recipient,inbox[0].ItemID);err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`UPDATE mail_items SET deleted_at=$2 WHERE mail_item_id=$1`,inbox[0].ItemID,old);err!=nil{t.Fatal(err)}
	if err=repo.PruneTrash(ctx,time.Now().UTC(),100);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_messages WHERE message_id=$1`,draft.ID).Scan(&messageCount);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_attachments WHERE attachment_id=$1`,attachment.ID).Scan(&attachmentCount);err!=nil{t.Fatal(err)}
	if messageCount!=0||attachmentCount!=0{t.Fatalf("fully expired message retained: message=%d attachment=%d",messageCount,attachmentCount)}
	if err=pool.QueryRow(ctx,`SELECT storage_used_bytes FROM mailboxes WHERE mailbox_id=$1`,senderBox.ID).Scan(&used);err!=nil{t.Fatal(err)};if used!=0{t.Fatalf("quota not released: %d",used)}
	var gc int;if err=pool.QueryRow(ctx,`SELECT count(*) FROM mail_blob_gc WHERE storage_key=(SELECT storage_key FROM mail_blob_gc ORDER BY queued_at DESC LIMIT 1)`).Scan(&gc);err!=nil{t.Fatal(err)};if gc<1{t.Fatal("blob gc was not queued")}
	if _,err=os.Stat(resolved.Path);err!=nil{t.Fatalf("blob disappeared before gc: %v",err)}
	if err=store.RunBlobGC(ctx,time.Now().UTC(),100);err!=nil{t.Fatal(err)}
	if _,err=os.Stat(resolved.Path);!errors.Is(err,os.ErrNotExist){t.Fatalf("blob remains after gc: %v",err)}
}
