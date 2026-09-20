package webmaster

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/venomimonstro/poisk/internal/crawler/fetcher"
)

type sitemapStoreFake struct {
	completed int
	urls []string
	children []string
}
func (s *sitemapStoreFake) LeaseSitemaps(context.Context,string,int,int)([]SitemapTask,error){return nil,nil}
func (s *sitemapStoreFake) RequeueExpiredSitemaps(context.Context)(int64,error){return 0,nil}
func (s *sitemapStoreFake) CompleteSitemap(_ context.Context,_ SitemapTask)error{s.completed++;return nil}
func (s *sitemapStoreFake) RetrySitemap(context.Context,SitemapTask,error,time.Duration)error{return nil}
func (s *sitemapStoreFake) AddChildSitemaps(_ context.Context,_ SitemapTask,urls []string)error{s.children=append([]string(nil),urls...);return nil}
func (s *sitemapStoreFake) AddSitemapURLs(_ context.Context,_ SitemapTask,urls []string)error{s.urls=append([]string(nil),urls...);return nil}

type sitemapFetcherFake struct{ result fetcher.Result; err error }
func (f sitemapFetcherFake) Fetch(context.Context,string,fetcher.Conditional)(fetcher.Result,error){return f.result,f.err}

func TestSitemapProcessorQueuesOwnedURLs(t *testing.T){
	body:=[]byte("<?xml version=\"1.0\"?><urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"><url><loc>https://example.com/a?utm_source=x</loc></url><url><loc>https://example.com/b</loc></url></urlset>")
	store:=&sitemapStoreFake{}
	p:=SitemapProcessor{Store:store,Fetcher:sitemapFetcherFake{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://example.com/sitemap.xml",Body:body}}}
	err:=p.Process(context.Background(),SitemapTask{ID:1,SiteID:1,DomainID:2,Host:"example.com",URL:"https://example.com/sitemap.xml"})
	if err!=nil{t.Fatal(err)}
	if store.completed!=1 || len(store.urls)!=2{t.Fatalf("completed=%d urls=%v",store.completed,store.urls)}
	if store.urls[0]!="https://example.com/a"{t.Fatalf("normalized=%q",store.urls[0])}
}

func TestSitemapProcessorQueuesChildSitemaps(t *testing.T){
	body:=[]byte("<?xml version=\"1.0\"?><sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"><sitemap><loc>https://example.com/a.xml</loc></sitemap></sitemapindex>")
	store:=&sitemapStoreFake{}
	p:=SitemapProcessor{Store:store,Fetcher:sitemapFetcherFake{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://example.com/sitemap.xml",Body:body}}}
	if err:=p.Process(context.Background(),SitemapTask{ID:1,SiteID:1,DomainID:2,Host:"example.com",URL:"https://example.com/sitemap.xml"});err!=nil{t.Fatal(err)}
	if store.completed!=1 || len(store.children)!=1 || store.children[0]!="https://example.com/a.xml"{t.Fatalf("children=%v completed=%d",store.children,store.completed)}
}

func TestSitemapProcessorRejectsCrossHostContent(t *testing.T){
	body:=[]byte("<?xml version=\"1.0\"?><urlset><url><loc>https://other.test/a</loc></url></urlset>")
	store:=&sitemapStoreFake{}
	p:=SitemapProcessor{Store:store,Fetcher:sitemapFetcherFake{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://example.com/sitemap.xml",Body:body}}}
	err:=p.Process(context.Background(),SitemapTask{ID:1,Host:"example.com",URL:"https://example.com/sitemap.xml"})
	if !errors.Is(err,ErrInvalidSiteOrigin){t.Fatalf("err=%v",err)}
	if store.completed!=0{t.Fatal("invalid sitemap completed")}
}

func TestSitemapProcessorRejectsCrossHostRedirect(t *testing.T){
	store:=&sitemapStoreFake{}
	p:=SitemapProcessor{Store:store,Fetcher:sitemapFetcherFake{result:fetcher.Result{StatusCode:http.StatusOK,FinalURL:"https://other.test/sitemap.xml",Body:[]byte("<urlset></urlset>")}}}
	if err:=p.Process(context.Background(),SitemapTask{ID:1,Host:"example.com",URL:"https://example.com/sitemap.xml"});err==nil{t.Fatal("cross-host redirect accepted")}
}
