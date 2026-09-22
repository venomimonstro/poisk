package mail

import (
	"errors"
	"strings"
	"testing"
)

func TestParseInboundMIMEPlainMultipartAndAttachment(t *testing.T){
	raw:=strings.Join([]string{
		"From: Sender <sender@example.com>",
		"Subject: =?UTF-8?Q?Test_=D0=BF=D0=B8=D1=81=D1=8C=D0=BC=D0=BE?=",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"hello=20world",
		"--abc",
		"Content-Type: text/plain",
		"Content-Disposition: attachment; filename=note.txt",
		"",
		"safe attachment",
		"--abc--",
		"",
	},"\r\n")
	msg,err:=ParseInboundMIME([]byte(raw));if err!=nil{t.Fatal(err)}
	if msg.From!="sender@example.com"{t.Fatalf("from=%q",msg.From)}
	if msg.BodyText!="hello world"{t.Fatalf("body=%q",msg.BodyText)}
	if len(msg.Attachments)!=1||msg.Attachments[0].Filename!="note.txt"{t.Fatalf("attachments=%+v",msg.Attachments)}
}

func TestParseInboundMIMEDiscardsActiveHTML(t *testing.T){
	raw:="From: a@example.com\r\nSubject: html\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<script>alert(1)</script><b>hello</b>"
	msg,err:=ParseInboundMIME([]byte(raw));if err!=nil{t.Fatal(err)}
	if strings.Contains(msg.BodyText,"script")||strings.Contains(msg.BodyText,"<b>"){t.Fatalf("active html leaked: %q",msg.BodyText)}
	if msg.BodyText==""{t.Fatal("expected safe fallback body")}
}

func TestParseInboundMIMERejectsDangerousAttachment(t *testing.T){
	raw:=strings.Join([]string{
		"From: a@example.com","Subject: dangerous","Content-Type: multipart/mixed; boundary=x","",
		"--x","Content-Type: text/plain","","body",
		"--x","Content-Type: application/octet-stream","Content-Disposition: attachment; filename=run.exe","","MZ",
		"--x--","",
	},"\r\n")
	_,err:=ParseInboundMIME([]byte(raw));if !errors.Is(err,ErrDangerousMailPart){t.Fatalf("expected dangerous part, got %v",err)}
}

func TestParseInboundMIMERejectsOversizedRaw(t *testing.T){
	raw:=make([]byte,MaxInboundRawBytes+1)
	if _,err:=ParseInboundMIME(raw);!errors.Is(err,ErrInvalid){t.Fatalf("expected invalid, got %v",err)}
}

func TestInboundRecipientAddressRejectsDisplayName(t *testing.T){
	if _,err:=InboundRecipientAddress("User <user@example.com>");!errors.Is(err,ErrInvalid){t.Fatalf("expected invalid, got %v",err)}
	if got,err:=InboundRecipientAddress("USER@Example.COM");err!=nil||got!="user@example.com"{t.Fatalf("got=%q err=%v",got,err)}
}
