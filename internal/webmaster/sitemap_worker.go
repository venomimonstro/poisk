package webmaster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/venomimonstro/poisk/internal/crawler/fetcher"
	"github.com/venomimonstro/poisk/internal/crawler/sitemap"
	"github.com/venomimonstro/poisk/internal/crawler/urlnorm"
)

type SitemapFetcher interface {
	Fetch(context.Context,string,fetcher.Conditional)(fetcher.Result,error)
}

type SitemapStore interface {
	LeaseSitemaps(context.Context,string,int,int)([]SitemapTask,error)
	RequeueExpiredSitemaps(context.Context)(int64,error)
	CompleteSitemap(context.Context,SitemapTask) error
	RetrySitemap(context.Context,SitemapTask,error,time.Duration) error
	AddChildSitemaps(context.Context,SitemapTask,[]string) error
	AddSitemapURLs(context.Context,SitemapTask,[]string) error
}

type SitemapProcessor struct {
	Store   SitemapStore
	Fetcher SitemapFetcher
	Limits  sitemap.Limits
}

func (p SitemapProcessor) Process(ctx context.Context,task SitemapTask) error {
	if p.Store==nil || p.Fetcher==nil{return errors.New("sitemap processor is not initialized")}
	limits:=p.Limits
	if limits.MaxCompressedBytes<=0{limits=sitemap.DefaultLimits()}
	result,err:=p.Fetcher.Fetch(ctx,task.URL,fetcher.Conditional{})
	if err!=nil{return err}
	if result.StatusCode<200 || result.StatusCode>=300{return fmt.Errorf("sitemap HTTP status %d",result.StatusCode)}
	if !sameRawHost(result.FinalURL,task.Host){return errors.New("sitemap redirected outside verified host")}
	compressed:=strings.HasSuffix(strings.ToLower(pathOnly(result.FinalURL)),".gz") || strings.Contains(strings.ToLower(result.Header.Get("Content-Type")),"gzip") || strings.Contains(strings.ToLower(result.Header.Get("Content-Encoding")),"gzip")
	parsed,err:=sitemap.Parse(bytes.NewReader(result.Body),compressed,task.Depth,limits)
	if err!=nil{return err}
	switch parsed.Kind{
	case sitemap.KindURLSet:
		urls,err:=ownedNormalizedURLs(task.Host,parsed.URLs); if err!=nil{return err}
		if err:=p.Store.AddSitemapURLs(ctx,task,urls);err!=nil{return err}
	case sitemap.KindSitemapIndex:
		children,err:=ownedNormalizedURLs(task.Host,parsed.Sitemaps); if err!=nil{return err}
		if err:=p.Store.AddChildSitemaps(ctx,task,children);err!=nil{return err}
	default:
		return sitemap.ErrUnknown
	}
	return p.Store.CompleteSitemap(ctx,task)
}

func ownedNormalizedURLs(host string,values []string)([]string,error){
	out:=make([]string,0,len(values)); seen:=make(map[string]struct{},len(values))
	for _,raw:=range values{
		u,err:=url.Parse(strings.TrimSpace(raw)); if err!=nil{return nil,err}
		if u.Scheme!="http" && u.Scheme!="https"{return nil,ErrInvalidSiteOrigin}
		if u.User!=nil || !sameSiteHost(u.Hostname(),host){return nil,ErrInvalidSiteOrigin}
		normalized,err:=urlnorm.Normalize(raw); if err!=nil{return nil,err}
		if _,ok:=seen[normalized];ok{continue}; seen[normalized]=struct{}{}
		out=append(out,normalized)
	}
	return out,nil
}

func sameRawHost(raw,host string)bool{u,err:=url.Parse(raw);return err==nil&&sameSiteHost(u.Hostname(),host)}
func pathOnly(raw string)string{u,err:=url.Parse(raw);if err!=nil{return ""};return u.Path}

type SitemapRunner struct {
	Store         SitemapStore
	Processor     SitemapProcessor
	WorkerID      string
	BatchSize     int
	LeaseSeconds  int
	PollInterval  time.Duration
}

func (r SitemapRunner) Run(ctx context.Context) error {
	if r.Store==nil || r.Processor.Store==nil || r.Processor.Fetcher==nil{return errors.New("sitemap runner is not initialized")}
	if r.WorkerID==""{return errors.New("sitemap worker id is required")}
	if r.BatchSize<=0{r.BatchSize=4}; if r.LeaseSeconds<=0{r.LeaseSeconds=30}; if r.PollInterval<=0{r.PollInterval=time.Second}
	ticker:=time.NewTicker(r.PollInterval);defer ticker.Stop()
	for{
		if _,err:=r.Store.RequeueExpiredSitemaps(ctx);err!=nil && ctx.Err()==nil{return err}
		tasks,err:=r.Store.LeaseSitemaps(ctx,r.WorkerID,r.BatchSize,r.LeaseSeconds);if err!=nil{if ctx.Err()!=nil{return ctx.Err()};return err}
		if len(tasks)==0{select{case<-ctx.Done():return ctx.Err();case<-ticker.C:continue}}
		for _,task:=range tasks{
			if err:=r.Processor.Process(ctx,task);err!=nil{
				delay:=time.Second<<minIntSitemap(task.Attempts-1,6);if delay>time.Minute{delay=time.Minute}
				if retryErr:=r.Store.RetrySitemap(ctx,task,err,delay);retryErr!=nil && ctx.Err()==nil{return errors.Join(err,retryErr)}
			}
		}
	}
}

func minIntSitemap(a,b int)int{if a<b{return a};return b}
