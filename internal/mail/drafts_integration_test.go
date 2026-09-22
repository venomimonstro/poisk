//go:build integration

package mail

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDraftEditIsTenantScopedAndKeepsReplyParent(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background()
	a:=mailUser(t,pool,"draft-a");b:=mailUser(t,pool,"draft-b");outsider:=mailUser(t,pool,"draft-outsider")
	boxA,err:=repo.EnsureMailbox(ctx,a);if err!=nil{t.Fatal(err)}
	boxB,err:=repo.EnsureMailbox(ctx,b);if err!=nil{t.Fatal(err)}
	if _,err=repo.EnsureMailbox(ctx,outsider);err!=nil{t.Fatal(err)}
	original,err:=repo.CreateDraft(ctx,a,DraftInput{Subject:"Parent message",BodyText:"Parent body used to create a sent message.",Recipients:[]Recipient{{Address:boxB.Address,Type:"TO"}}});if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,a,original.ID,time.Now().UTC());err!=nil{t.Fatal(err)}
	inbox,err:=repo.ListFolder(ctx,b,"INBOX",10,0);if err!=nil||len(inbox)!=1{t.Fatalf("inbox=%+v err=%v",inbox,err)}
	reply,err:=repo.CreateDraft(ctx,b,DraftInput{Subject:"Re: Parent message",BodyText:"Reply body that must keep its parent link.",Recipients:[]Recipient{{Address:boxA.Address,Type:"TO"}},Provenance:"REPLY",ParentMessageID:&original.ID});if err!=nil{t.Fatal(err)}
	edit,err:=repo.GetDraftEdit(ctx,b,reply.ID);if err!=nil{t.Fatal(err)}
	if edit.Provenance!="REPLY"||edit.ParentMessageID==nil||*edit.ParentMessageID!=original.ID||edit.ThreadID!=original.ThreadID{t.Fatalf("draft edit=%+v original=%+v",edit,original)}
	if len(edit.Recipients)!=1||edit.Recipients[0].Address!=boxA.Address{t.Fatalf("recipients=%+v",edit.Recipients)}
	if _,err=repo.GetDraftEdit(ctx,outsider,reply.ID);!errors.Is(err,ErrNotFound){t.Fatalf("outsider draft read err=%v",err)}
}
