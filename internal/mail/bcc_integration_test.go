//go:build integration

package mail

import (
	"context"
	"testing"
	"time"
)

func TestBCCPrivacyByViewer(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background()
	sender:=mailUser(t,pool,"bcc-sender");toUser:=mailUser(t,pool,"bcc-to");bccUser:=mailUser(t,pool,"bcc-hidden")
	toBox,err:=repo.EnsureMailbox(ctx,toUser);if err!=nil{t.Fatal(err)}
	bccBox,err:=repo.EnsureMailbox(ctx,bccUser);if err!=nil{t.Fatal(err)}
	draft,err:=repo.CreateDraft(ctx,sender,DraftInput{Subject:"BCC privacy",BodyText:"BCC address must not leak to ordinary recipients.",Recipients:[]Recipient{{Address:toBox.Address,Type:"TO"},{Address:bccBox.Address,Type:"BCC"}}});if err!=nil{t.Fatal(err)}
	if _,err=repo.SendDraft(ctx,sender,draft.ID,time.Now().UTC());err!=nil{t.Fatal(err)}

	sent,err:=repo.ListFolder(ctx,sender,"SENT",10,0);if err!=nil||len(sent)!=1{t.Fatalf("sent=%+v err=%v",sent,err)}
	senderItem,err:=repo.GetItem(ctx,sender,sent[0].ItemID);if err!=nil{t.Fatal(err)}
	if len(senderItem.Message.Recipients)!=2{t.Fatalf("sender recipients=%+v",senderItem.Message.Recipients)}

	toInbox,err:=repo.ListFolder(ctx,toUser,"INBOX",10,0);if err!=nil||len(toInbox)!=1{t.Fatalf("to inbox=%+v err=%v",toInbox,err)}
	toItem,err:=repo.GetItem(ctx,toUser,toInbox[0].ItemID);if err!=nil{t.Fatal(err)}
	if len(toItem.Message.Recipients)!=1||toItem.Message.Recipients[0].Address!=toBox.Address||toItem.Message.Recipients[0].Type!="TO"{t.Fatalf("TO viewer leaked BCC: %+v",toItem.Message.Recipients)}

	bccInbox,err:=repo.ListFolder(ctx,bccUser,"INBOX",10,0);if err!=nil||len(bccInbox)!=1{t.Fatalf("bcc inbox=%+v err=%v",bccInbox,err)}
	bccItem,err:=repo.GetItem(ctx,bccUser,bccInbox[0].ItemID);if err!=nil{t.Fatal(err)}
	seenOwnBCC:=false;for _,r:=range bccItem.Message.Recipients{if r.Type=="BCC"{if r.Address!=bccBox.Address{t.Fatalf("BCC viewer saw foreign bcc: %+v",bccItem.Message.Recipients)};seenOwnBCC=true}}
	if !seenOwnBCC{t.Fatalf("BCC viewer cannot see own address: %+v",bccItem.Message.Recipients)}
}
