package mail

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeDNSResolver struct {
	mx map[string][]*net.MX
	txt map[string][]string
	err map[string]error
}

func (f fakeDNSResolver) LookupMX(_ context.Context,name string)([]*net.MX,error){if err:=f.err["mx:"+name];err!=nil{return nil,err};return f.mx[name],nil}
func (f fakeDNSResolver) LookupTXT(_ context.Context,name string)([]string,error){if err:=f.err["txt:"+name];err!=nil{return nil,err};return f.txt[name],nil}

func TestCheckInternetMailDNSReady(t *testing.T){
	resolver:=fakeDNSResolver{
		mx:map[string][]*net.MX{"example.test":{{Host:"mail.example.test.",Pref:10}}},
		txt:map[string][]string{
			"example.test":{"v=spf1 ip4:192.0.2.10 -all"},
			"_dmarc.example.test":{"v=DMARC1; p=reject"},
			"poisk1._domainkey.example.test":{"v=DKIM1; k=rsa; p=abc123"},
		},
		err:map[string]error{},
	}
	out,err:=CheckInternetMailDNS(context.Background(),resolver,"Example.Test.","poisk1","v=DKIM1; k=rsa; p=abc123")
	if err!=nil{t.Fatal(err)}
	if !out.Ready||!out.MX||!out.SPF||!out.DMARC||!out.DKIM{t.Fatalf("readiness=%+v",out)}
	if len(out.Reasons)!=0{t.Fatalf("unexpected reasons=%v",out.Reasons)}
}

func TestCheckInternetMailDNSReportsMissingAndMismatch(t *testing.T){
	resolver:=fakeDNSResolver{
		mx:map[string][]*net.MX{},
		txt:map[string][]string{
			"example.test":{"google-site-verification=x"},
			"_dmarc.example.test":{"not-dmarc"},
			"poisk1._domainkey.example.test":{"v=DKIM1; k=rsa; p=wrong"},
		},
		err:map[string]error{},
	}
	out,err:=CheckInternetMailDNS(context.Background(),resolver,"example.test","poisk1","v=DKIM1;k=rsa;p=expected")
	if err!=nil{t.Fatal(err)}
	if out.Ready{t.Fatalf("unexpected ready: %+v",out)}
	want:=map[string]bool{"mx_missing":false,"spf_missing":false,"dmarc_missing":false,"dkim_missing_or_mismatch":false}
	for _,reason:=range out.Reasons{if _,ok:=want[reason];ok{want[reason]=true}}
	for reason,seen:=range want{if !seen{t.Fatalf("missing reason %q in %v",reason,out.Reasons)}}
}

func TestCheckInternetMailDNSPropagatesResolverFailure(t *testing.T){
	resolver:=fakeDNSResolver{mx:map[string][]*net.MX{},txt:map[string][]string{},err:map[string]error{"mx:example.test":errors.New("resolver unavailable")}}
	if _,err:=CheckInternetMailDNS(context.Background(),resolver,"example.test","poisk1","");err==nil{t.Fatal("expected resolver failure")}
}
