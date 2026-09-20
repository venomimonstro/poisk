package webmaster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/venomimonstro/poisk/internal/crawler/fetcher"
)

const (
	VerificationDNS  = "DNS_TXT"
	VerificationFile = "HTML_FILE"
	VerificationMeta = "META_TAG"
)

var ErrVerificationFailed = errors.New("ownership verification failed")

type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type ProofFetcher interface {
	Fetch(ctx context.Context, rawURL string, conditional fetcher.Conditional) (fetcher.Result, error)
}

type NetTXTResolver struct{}
func (NetTXTResolver) LookupTXT(ctx context.Context, name string) ([]string,error) { return net.DefaultResolver.LookupTXT(ctx,name) }

type Verifier struct {
	DNS     TXTResolver
	Fetcher ProofFetcher
}

func (v Verifier) Verify(ctx context.Context, site Site, method, token string) error {
	if site.Host=="" || token=="" { return ErrVerificationFailed }
	switch method {
	case VerificationDNS:
		resolver:=v.DNS
		if resolver==nil { resolver=NetTXTResolver{} }
		records,err:=resolver.LookupTXT(ctx,"_poisk-verification."+site.Host)
		if err!=nil { return fmt.Errorf("lookup ownership TXT: %w",err) }
		want:="poisk-verification="+token
		for _,record:=range records { if strings.TrimSpace(record)==want { return nil } }
		return ErrVerificationFailed
	case VerificationFile:
		if v.Fetcher==nil { return errors.New("ownership proof fetcher is not configured") }
		proofURL:=site.Origin+"/.well-known/poisk-verification/"+url.PathEscape(token)+".txt"
		result,err:=v.Fetcher.Fetch(ctx,proofURL,fetcher.Conditional{})
		if err!=nil { return err }
		if result.StatusCode!=200 || !sameHost(result.FinalURL,site.Host) { return ErrVerificationFailed }
		if strings.TrimSpace(string(result.Body))!=token { return ErrVerificationFailed }
		return nil
	case VerificationMeta:
		if v.Fetcher==nil { return errors.New("ownership proof fetcher is not configured") }
		result,err:=v.Fetcher.Fetch(ctx,site.Origin+"/",fetcher.Conditional{})
		if err!=nil { return err }
		if result.StatusCode!=200 || !sameHost(result.FinalURL,site.Host) { return ErrVerificationFailed }
		if hasVerificationMeta(result.Body,token) { return nil }
		return ErrVerificationFailed
	default:
		return ErrVerificationFailed
	}
}

func sameHost(raw, expected string) bool {
	u,err:=url.Parse(raw)
	if err!=nil { return false }
	return strings.EqualFold(strings.TrimSuffix(u.Hostname(),"."),strings.TrimSuffix(expected,"."))
}

func hasVerificationMeta(body []byte, token string) bool {
	doc,err:=html.Parse(strings.NewReader(string(body)))
	if err!=nil { return false }
	var walk func(*html.Node) bool
	walk=func(n *html.Node) bool {
		if n.Type==html.ElementNode && strings.EqualFold(n.Data,"meta") {
			var name,content string
			for _,a:=range n.Attr {
				switch strings.ToLower(a.Key) { case "name": name=a.Val; case "content": content=a.Val }
			}
			if strings.EqualFold(strings.TrimSpace(name),"poisk-verification") && strings.TrimSpace(content)==token { return true }
		}
		for child:=n.FirstChild;child!=nil;child=child.NextSibling { if walk(child) { return true } }
		return false
	}
	return walk(doc)
}
