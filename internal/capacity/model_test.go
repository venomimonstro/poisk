package capacity

import (
	"testing"
	"time"
)

func TestComputeWorkloadPercentilesAndErrorRate(t *testing.T){
	samples:=[]time.Duration{10*time.Millisecond,20*time.Millisecond,30*time.Millisecond,40*time.Millisecond,50*time.Millisecond,60*time.Millisecond,70*time.Millisecond,80*time.Millisecond,90*time.Millisecond,100*time.Millisecond}
	m:=ComputeWorkload(samples,2,time.Second)
	if m.Requests!=10||m.Errors!=2{t.Fatalf("counts=%+v",m)}
	if m.ErrorRate!=0.2{t.Fatalf("error_rate=%f",m.ErrorRate)}
	if m.QPS!=10{t.Fatalf("qps=%f",m.QPS)}
	if m.P50MS!=50||m.P95MS!=100||m.P99MS!=100{t.Fatalf("percentiles=%+v",m)}
}

func TestProjectionIsExplicitlyProjected(t *testing.T){
	p,err:=ProjectStorage(1_000_000,10_000_000,StorageSnapshot{DatabaseBytes:20_000_000_000,ManticoreBytes:30_000_000_000})
	if err!=nil{t.Fatal(err)}
	if p.Kind!="PROJECTED_LINEAR_STORAGE_ONLY"||p.ScaleFactor!=10{t.Fatalf("projection=%+v",p)}
	if p.ProjectedTotalBytes!=500_000_000_000{t.Fatalf("bytes=%d",p.ProjectedTotalBytes)}
	if p.Note==""{t.Fatal("projection must state its limitation")}
}

func TestProjectionRejectsMissingMeasuredCorpus(t *testing.T){
	if _,err:=ProjectStorage(0,10_000_000,StorageSnapshot{});err==nil{t.Fatal("expected error")}
}

func TestClassifyCapacitySignals(t *testing.T){
	got:=Classify(Signals{Search:WorkloadMetrics{QPS:40,P95MS:1200,ErrorRate:.02},CPUPercent:90,RAMPercent:92,DiskPercent:91,CrawlReady:60000,OutboxReady:20000,TargetQPS:100,ProjectedBytes:900,AvailableDiskBytes:1000})
	want:=map[string]bool{"SEARCH_SLO":true,"CPU_PRESSURE":true,"RAM_PRESSURE":true,"DISK_PRESSURE":true,"CRAWL_BACKLOG":true,"INDEX_BACKLOG":true,"QPS_CAPACITY":true,"PROJECTED_STORAGE":true}
	for _,b:=range got{delete(want,b.Code)}
	if len(want)!=0{t.Fatalf("missing bottlenecks=%v got=%+v",want,got)}
}
