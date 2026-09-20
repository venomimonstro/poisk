package webmaster

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/venomimonstro/poisk/internal/crawler/fetcher"
)

type fakeTXT struct { records []string; err error }
func (f fakeTXT) LookupTXT(context.Context,string)([]string,error){ return f.records,f.err }

type fakeProofFetcher struct { result fetcher.Result; err error; requested string }
func (f *fakeProofFetcher) Fetch(_ context.Context, raw string, _ fetcher.Conditional)(fetcher.Result,error){ f.requested=raw; return f.result,f.err }

func TestVerifyDNSRequiresExactToken(t *testing.T){
	v:=Verifier{DNS:fakeTXT{records:[]string{"other=x","poisk-verification=secret"}}}
	site:=Site{Host:"example.com",Origin:"https://example.com"}
	if err:=v.Verify(context.Background(),site,VerificationDNS,"secret"); err!=nil { t.Fatal(err) }
	if err:=v.Verify(context.Background(),site,VerificationDNS,"wrong"); !errors.Is(err,ErrVerificationFailed){ t.Fatalf("err=%v",err) }
}

func TestVerifyFileRejectsCrossHostRedirect(t *testing.T){
	f:=&fakeProofFetcher{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://attacker.test/proof.txt",Body:[]byte("secret")}}
	v:=Verifier{Fetcher:f}
	err:=v.Verify(context.Background(),Site{Host:"example.com",Origin:"https://example.com"},VerificationFile,"secret")
	if !errors.Is(err,ErrVerificationFailed){ t.Fatalf("err=%v",err) }
}

func TestVerifyFileRequiresExactBody(t *testing.T){
	f:=&fakeProofFetcher{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://example.com/.well-known/x",Body:[]byte(" secret \n")}}
	v:=Verifier{Fetcher:f}
	if err:=v.Verify(context.Background(),Site{Host:"example.com",Origin:"https://example.com"},VerificationFile,"secret"); err!=nil { t.Fatal(err) }
}

func TestVerifyMetaParsesAttributeOrderAndEscaping(t *testing.T){
	body:=[]byte(`<html><head><meta content="secret" data-x="1" name="poisk-verification"></head></html>`)
	f:=&fakeProofFetcher{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://example.com/",Body:body}}
	v:=Verifier{Fetcher:f}
	if err:=v.Verify(context.Background(),Site{Host:"example.com",Origin:"https://example.com"},VerificationMeta,"secret"); err!=nil { t.Fatal(err) }
}

func TestVerifyUnknownMethodRejected(t *testing.T){
	if err:= (Verifier{}).Verify(context.Background(),Site{Host:"example.com"},"BAD","x"); !errors.Is(err,ErrVerificationFailed){ t.Fatalf("err=%v",err) }
}
