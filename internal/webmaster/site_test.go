package webmaster

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
)

type wmResolver map[string][]netip.Addr
func (r wmResolver) LookupNetIP(_ context.Context,_ string,host string)([]netip.Addr,error){
	ips,ok:=r[host]; if !ok { return nil,errors.New("not found") }; return ips,nil
}

func TestValidateSiteOriginAllowsOnlyPublicRootOrigin(t *testing.T){
	v:=crawlersecurity.NewValidator()
	v.Resolver=wmResolver{"example.com":{netip.MustParseAddr("1.1.1.1")}}
	got,err:=ValidateSiteOrigin(context.Background(),v,"https://example.com/")
	if err!=nil { t.Fatal(err) }
	if got.Origin!="https://example.com" || got.Host!="example.com" { t.Fatalf("origin=%+v",got) }
	for _,raw:=range []string{
		"https://example.com/path",
		"https://example.com/?x=1",
		"https://example.com/#x",
		"https://user@example.com/",
		"https://example.com:8443/",
	} {
		if _,err:=ValidateSiteOrigin(context.Background(),v,raw); err==nil { t.Fatalf("expected rejection for %s",raw) }
	}
}

func TestValidateSiteOriginRejectsPrivateDNS(t *testing.T){
	v:=crawlersecurity.NewValidator()
	v.Resolver=wmResolver{"private.test":{netip.MustParseAddr("10.0.0.1")}}
	if _,err:=ValidateSiteOrigin(context.Background(),v,"https://private.test/"); err==nil { t.Fatal("private origin accepted") }
}
