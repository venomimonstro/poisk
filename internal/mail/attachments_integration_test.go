//go:build integration

package mail

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAttachmentTenantIsolationTraversalAndGC(t *testing.T){
	pool:=mailDB(t);ctx:=context.Background();repo:=Repository{DB:pool};store:=AttachmentStore{Repo:repo,Root:t.TempDir()}
	sender:=mailUser(t,pool,"attach-sender");recipient:=mailUser(t,pool,"attach-recipient");outsider:=mailUser(t,pool,"attach-outsider")
	recipientBox,err:=repo.EnsureMailbox(ctx,recipient);if err!=nil{t.Fatal(err)};if _,err=repo.EnsureMailbox(ctx,outsider);err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Attachment isolation",BodyText:"Attachment tenant isolation integration test body.",Recipients:[]Recipient{{Address:recipientBox.Address,Type:"TO"}}});if err!=nil{t.Fatal(err)}
	if _,err=store.Upload(ctx,sender,draft.ID,"../escape.txt","text/plain",bytes.NewBufferString("x"),time.Now().UTC());!errors.Is(err,ErrInvalid){t.Fatalf("traversal filename err=%v",err)}
	payload:=[]byte("tenant-safe attachment payload")
	attachment,err:=store.Upload(ctx,sender,draft.ID,"evidence.txt","text/plain",bytes.NewReader(payload),time.Now().UTC());if err!=nil{t.Fatal(err)}
	resolved,err:=store.ResolveDownload(ctx,sender,attachment.ID);if err!=nil{t.Fatal(err)}
	if filepath.Dir(resolved.Path)!=store.Root{t.Fatalf("path escaped root: %s",resolved.Path)}
	if filepath.Base(resolved.Path)==attachment.Filename{t.Fatal("original filename used as storage path")}
	if _,err=store.ResolveDownload(ctx,outsider,attachment.ID);!errors.Is(err,ErrNotFound){t.Fatalf("outsider resolve err=%v",err)}
	if err=store.Detach(ctx,outsider,draft.ID,attachment.ID);!errors.Is(err,ErrNotFound){t.Fatalf("outsider detach err=%v",err)}
	var before int64;if err=pool.QueryRow(ctx,`SELECT storage_used_bytes FROM mailboxes WHERE consumer_user_id=$1`,sender).Scan(&before);err!=nil{t.Fatal(err)};if before!=int64(len(payload)){t.Fatalf("storage before=%d",before)}
	if err=store.Detach(ctx,sender,draft.ID,attachment.ID);err!=nil{t.Fatal(err)}
	var after int64;if err=pool.QueryRow(ctx,`SELECT storage_used_bytes FROM mailboxes WHERE consumer_user_id=$1`,sender).Scan(&after);err!=nil{t.Fatal(err)};if after!=0{t.Fatalf("storage after=%d",after)}
	if _,err=os.Stat(resolved.Path);err!=nil{t.Fatalf("blob should await gc: %v",err)}
	if err=store.RunBlobGC(ctx,time.Now().UTC(),10);err!=nil{t.Fatal(err)}
	if _,err=os.Stat(resolved.Path);!errors.Is(err,os.ErrNotExist){t.Fatalf("blob still present after gc: %v",err)}
}

func TestDeliveredAttachmentVisibleOnlyToParticipants(t *testing.T){
	pool:=mailDB(t);ctx:=context.Background();repo:=Repository{DB:pool};store:=AttachmentStore{Repo:repo,Root:t.TempDir()}
	sender:=mailUser(t,pool,"attach-delivery-sender");recipient:=mailUser(t,pool,"attach-delivery-recipient");outsider:=mailUser(t,pool,"attach-delivery-outsider")
	recipientBox,err:=repo.EnsureMailbox(ctx,recipient);if err!=nil{t.Fatal(err)};if _,err=repo.EnsureMailbox(ctx,outsider);err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"Delivered attachment",BodyText:"Recipient can access attachment only after delivery.",Recipients:[]Recipient{{Address:recipientBox.Address,Type:"TO"}}});if err!=nil{t.Fatal(err)}
	attachment,err:=store.Upload(ctx,sender,draft.ID,"photo.bin","application/octet-stream",bytes.NewReader([]byte{1,2,3,4}),time.Now().UTC());if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	if _,err=store.ResolveDownload(ctx,recipient,attachment.ID);err!=nil{t.Fatalf("recipient resolve: %v",err)}
	if _,err=store.ResolveDownload(ctx,outsider,attachment.ID);!errors.Is(err,ErrNotFound){t.Fatalf("outsider resolve err=%v",err)}
}
