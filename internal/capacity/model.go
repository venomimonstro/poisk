package capacity

import (
	"errors"
	"math"
	"sort"
	"time"
)

var ErrInvalid=errors.New("invalid capacity benchmark input")

type WorkloadMetrics struct{
	Requests int64 `json:"requests"`
	Errors int64 `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
	QPS float64 `json:"qps"`
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
	P99MS float64 `json:"p99_ms"`
	MaxMS float64 `json:"max_ms"`
	ElapsedMS int64 `json:"elapsed_ms"`
}

type StorageSnapshot struct{
	DatabaseBytes int64 `json:"database_bytes"`
	ManticoreBytes int64 `json:"manticore_bytes"`
	MapBytes int64 `json:"map_bytes"`
	BackupBytes int64 `json:"backup_bytes"`
}

type Projection struct{
	Kind string `json:"kind"`
	MeasuredDocuments int64 `json:"measured_documents"`
	TargetDocuments int64 `json:"target_documents"`
	ScaleFactor float64 `json:"scale_factor"`
	ProjectedDatabaseBytes int64 `json:"projected_database_bytes"`
	ProjectedManticoreBytes int64 `json:"projected_manticore_bytes"`
	ProjectedTotalBytes int64 `json:"projected_total_bytes"`
	Note string `json:"note"`
}

type Signals struct{
	Search WorkloadMetrics `json:"search"`
	GEO WorkloadMetrics `json:"geo"`
	Address WorkloadMetrics `json:"address"`
	CPUPercent float64 `json:"cpu_percent"`
	RAMPercent float64 `json:"ram_percent"`
	DiskPercent float64 `json:"disk_percent"`
	CrawlReady int64 `json:"crawl_ready"`
	OutboxReady int64 `json:"outbox_ready"`
	CrawlPerSecond float64 `json:"crawl_per_second"`
	IndexPerSecond float64 `json:"index_per_second"`
	TargetQPS float64 `json:"target_qps"`
	ProjectedBytes int64 `json:"projected_bytes"`
	AvailableDiskBytes int64 `json:"available_disk_bytes"`
}

type Bottleneck struct{Code string `json:"code"`;Severity string `json:"severity"`;Detail string `json:"detail"`}

func ComputeWorkload(samples []time.Duration,errorsCount int64,elapsed time.Duration)WorkloadMetrics{
	if elapsed<=0{elapsed=time.Nanosecond}
	clean:=make([]time.Duration,0,len(samples));for _,d:=range samples{if d<0{d=0};clean=append(clean,d)}
	sort.Slice(clean,func(i,j int)bool{return clean[i]<clean[j]})
	requests:=int64(len(clean));if errorsCount<0{errorsCount=0};if errorsCount>requests{errorsCount=requests}
	metric:=WorkloadMetrics{Requests:requests,Errors:errorsCount,ElapsedMS:elapsed.Milliseconds()}
	metric.QPS=float64(requests)/elapsed.Seconds();if requests>0{metric.ErrorRate=float64(errorsCount)/float64(requests);metric.P50MS=durationMS(percentile(clean,.50));metric.P95MS=durationMS(percentile(clean,.95));metric.P99MS=durationMS(percentile(clean,.99));metric.MaxMS=durationMS(clean[len(clean)-1])}
	return metric
}

func percentile(sorted []time.Duration,p float64)time.Duration{
	if len(sorted)==0{return 0};if p<=0{return sorted[0]};if p>=1{return sorted[len(sorted)-1]}
	idx:=int(math.Ceil(p*float64(len(sorted))))-1;if idx<0{idx=0};if idx>=len(sorted){idx=len(sorted)-1};return sorted[idx]
}
func durationMS(d time.Duration)float64{return float64(d.Microseconds())/1000}

func ProjectStorage(measuredDocuments,targetDocuments int64,s StorageSnapshot)(Projection,error){
	if measuredDocuments<=0||targetDocuments<=0||targetDocuments>100_000_000||s.DatabaseBytes<0||s.ManticoreBytes<0{return Projection{},ErrInvalid}
	factor:=float64(targetDocuments)/float64(measuredDocuments)
	db:=scaledBytes(s.DatabaseBytes,factor);idx:=scaledBytes(s.ManticoreBytes,factor)
	return Projection{Kind:"PROJECTED_LINEAR_STORAGE_ONLY",MeasuredDocuments:measuredDocuments,TargetDocuments:targetDocuments,ScaleFactor:factor,ProjectedDatabaseBytes:db,ProjectedManticoreBytes:idx,ProjectedTotalBytes:safeAdd(db,idx),Note:"Projection is not a measured 10M result. It linearly scales measured database/index bytes per document and must be replaced by a real benchmark before an architecture change."},nil
}
func scaledBytes(v int64,f float64)int64{if v<=0||f<=0{return 0};x:=float64(v)*f;if x>float64(math.MaxInt64){return math.MaxInt64};return int64(math.Ceil(x))}
func safeAdd(a,b int64)int64{if a>math.MaxInt64-b{return math.MaxInt64};return a+b}

func Classify(s Signals)[]Bottleneck{
	out:=make([]Bottleneck,0,8)
	if s.Search.P95MS>1000||s.Search.ErrorRate>0.01{out=append(out,Bottleneck{"SEARCH_SLO","HIGH","Search P95 exceeds 1000ms or error rate exceeds 1%."})}else if s.Search.P95MS>500{out=append(out,Bottleneck{"SEARCH_LATENCY","MEDIUM","Search P95 exceeds 500ms."})}
	if s.CPUPercent>=85{out=append(out,Bottleneck{"CPU_PRESSURE","HIGH","Measured CPU utilization is at or above 85%."})}else if s.CPUPercent>=70{out=append(out,Bottleneck{"CPU_PRESSURE","MEDIUM","Measured CPU utilization is at or above 70%."})}
	if s.RAMPercent>=90{out=append(out,Bottleneck{"RAM_PRESSURE","HIGH","Measured memory utilization is at or above 90%."})}else if s.RAMPercent>=80{out=append(out,Bottleneck{"RAM_PRESSURE","MEDIUM","Measured memory utilization is at or above 80%."})}
	if s.DiskPercent>=90{out=append(out,Bottleneck{"DISK_PRESSURE","HIGH","Measured disk utilization is at or above 90%."})}else if s.DiskPercent>=80{out=append(out,Bottleneck{"DISK_PRESSURE","MEDIUM","Measured disk utilization is at or above 80%."})}
	if s.OutboxReady>10000{out=append(out,Bottleneck{"INDEX_BACKLOG","HIGH","Index outbox READY/RETRY backlog exceeds 10,000 events."})}
	if s.CrawlReady>50000{out=append(out,Bottleneck{"CRAWL_BACKLOG","MEDIUM","Crawler READY/RETRY backlog exceeds 50,000 URLs."})}
	if s.TargetQPS>0&&s.Search.QPS>0&&s.Search.QPS<s.TargetQPS{out=append(out,Bottleneck{"QPS_CAPACITY","HIGH","Measured Search QPS is below the declared target QPS."})}
	if s.ProjectedBytes>0&&s.AvailableDiskBytes>0&&s.ProjectedBytes>int64(float64(s.AvailableDiskBytes)*0.80){out=append(out,Bottleneck{"PROJECTED_STORAGE","HIGH","Projected 10M index+database bytes exceed 80% of currently available disk."})}
	return out
}
