package mail

import "testing"

func TestNormalizeExternalAddress(t *testing.T){
	valid:=map[string]string{
		"user@example.com":"user@example.com",
		"User.Name+tag@Example.COM":"User.Name+tag@example.com",
	}
	for raw,want:=range valid{got,err:=NormalizeExternalAddress(raw);if err!=nil||got!=want{t.Fatalf("%q -> %q err=%v want=%q",raw,got,err,want)}}
	bad:=[]string{"","a","name <user@example.com>","user@internal.poisk","user@example.com\r\nBcc:evil@example.com","user name@example.com","user@localhost","user@example.com."}
	for _,raw:=range bad{if got,err:=NormalizeExternalAddress(raw);err==nil{t.Fatalf("invalid external address accepted: %q -> %q",raw,got)}}
}

func TestOutboundIdempotencyDeterministicAndRecipientScoped(t *testing.T){
	a:=outboundIdempotency(10,20,"User@Example.com")
	b:=outboundIdempotency(10,20,"user@example.com")
	if a!=b{t.Fatalf("case-normalized address changed idempotency key: %s != %s",a,b)}
	if a==outboundIdempotency(10,21,"user@example.com"){t.Fatal("different recipient id produced same key")}
	if a==outboundIdempotency(11,20,"user@example.com"){t.Fatal("different message id produced same key")}
	if len(a)!=64{t.Fatalf("key length=%d",len(a))}
}
