package capacity

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const maxLatencySamples = 1_000_000

type HTTPBenchmarkConfig struct{
	URLs []string
	Duration time.Duration
	Concurrency int
	RequestTimeout time.Duration
	MaxBodyBytes int64
}

func RunHTTPBenchmark(ctx context.Context,cfg HTTPBenchmarkConfig)(WorkloadMetrics,error){
	if len(cfg.URLs)==0||len(cfg.URLs)>10000||cfg.Duration<=0||cfg.Duration>30*time.Minute||cfg.Concurrency<1||cfg.Concurrency>512{return WorkloadMetrics{},ErrInvalid}
	if cfg.RequestTimeout<=0{cfg.RequestTimeout=3*time.Second};if cfg.RequestTimeout>30*time.Second{return WorkloadMetrics{},ErrInvalid}
	if cfg.MaxBodyBytes<=0{cfg.MaxBodyBytes=128<<10};if cfg.MaxBodyBytes>4<<20{return WorkloadMetrics{},ErrInvalid}
	for _,raw:=range cfg.URLs{u,err:=url.Parse(raw);if err!=nil||u.Host==""||(u.Scheme!="http"&&u.Scheme!="https"){return WorkloadMetrics{},ErrInvalid}}

	benchCtx,cancel:=context.WithTimeout(ctx,cfg.Duration);defer cancel()
	client:=&http.Client{Timeout:cfg.RequestTimeout,CheckRedirect:func(_ *http.Request,_ []*http.Request)error{return http.ErrUseLastResponse}}
	start:=time.Now();samples:=make([]time.Duration,0,minIntBench(maxLatencySamples,cfg.Concurrency*4096));var mu sync.Mutex;var requests atomic.Int64;var errorsCount atomic.Int64;var sequence atomic.Uint64
	var wg sync.WaitGroup;wg.Add(cfg.Concurrency)
	for worker:=0;worker<cfg.Concurrency;worker++{go func(){defer wg.Done();for{
			if benchCtx.Err()!=nil{return}
			i:=sequence.Add(1)-1;target:=cfg.URLs[int(i%uint64(len(cfg.URLs)))];req,err:=http.NewRequestWithContext(benchCtx,http.MethodGet,target,nil);if err!=nil{errorsCount.Add(1);requests.Add(1);continue};req.Header.Set("Accept","application/json")
			began:=time.Now();resp,err:=client.Do(req);latency:=time.Since(began)
			if err!=nil&&benchCtx.Err()!=nil{return}
			failed:=err!=nil
			if err==nil{
				if resp.ContentLength>cfg.MaxBodyBytes&&resp.ContentLength>=0{failed=true}
				n,readErr:=io.Copy(io.Discard,io.LimitReader(resp.Body,cfg.MaxBodyBytes+1));_ = resp.Body.Close()
				if readErr!=nil&&benchCtx.Err()!=nil{return}
				if readErr!=nil||n>cfg.MaxBodyBytes||resp.StatusCode<200||resp.StatusCode>=300{failed=true}
			}
			requests.Add(1);if failed{errorsCount.Add(1)}
			mu.Lock();if len(samples)<maxLatencySamples{samples=append(samples,latency)};mu.Unlock()
		}
	}()}
	wg.Wait();elapsed:=time.Since(start);return ComputeWorkloadCounts(samples,requests.Load(),errorsCount.Load(),elapsed),nil
}

func minIntBench(a,b int)int{if a<b{return a};return b}
