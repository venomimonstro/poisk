package capacity

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"context"
)

func TestRunHTTPBenchmarkCountsRequestsAndErrors(t *testing.T){
	var n int
	srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){n++;if n%3==0{w.WriteHeader(http.StatusServiceUnavailable);return};w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"ok":true}`))}))
	defer srv.Close()
	m,err:=RunHTTPBenchmark(context.Background(),HTTPBenchmarkConfig{URLs:[]string{srv.URL},Duration:80*time.Millisecond,Concurrency:2,RequestTimeout:time.Second,MaxBodyBytes:1024})
	if err!=nil{t.Fatal(err)}
	if m.Requests<=0||m.SampledRequests<=0{t.Fatalf("metrics=%+v",m)}
	if m.Errors<=0||m.ErrorRate<=0{t.Fatalf("expected errors: %+v",m)}
	if m.QPS<=0{t.Fatalf("qps=%f",m.QPS)}
}

func TestRunHTTPBenchmarkRejectsUnboundedInputs(t *testing.T){
	if _,err:=RunHTTPBenchmark(context.Background(),HTTPBenchmarkConfig{URLs:[]string{"http://example.test"},Duration:time.Hour,Concurrency:1});err==nil{t.Fatal("expected duration rejection")}
	if _,err:=RunHTTPBenchmark(context.Background(),HTTPBenchmarkConfig{URLs:[]string{"file:///tmp/x"},Duration:time.Second,Concurrency:1});err==nil{t.Fatal("expected scheme rejection")}
}
