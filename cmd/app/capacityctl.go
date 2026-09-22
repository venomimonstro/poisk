package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/capacity"
)

func runCapacityCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: capacityctl benchmark <label> | status | adr <snapshot_id> <choice> <decided_by> <rationale>")}
	repo:=capacity.NewRepository(pool)
	switch args[0]{
	case "benchmark":
		if len(args)<2{return errors.New("usage: capacityctl benchmark <label>")}
		return runCapacityBenchmark(ctx,repo,args[1])
	default:
		return fmt.Errorf("capacityctl command %q is not implemented",args[0])
	}
}

func runCapacityBenchmark(ctx context.Context,repo *capacity.Repository,label string)error{
	mode:=strings.ToUpper(strings.TrimSpace(envDefault("CAPACITY_MODE","LIVE_READONLY")))
	base:=strings.TrimRight(envDefault("CAPACITY_BASE_URL","http://backend:8080"),"/")
	duration,err:=envDurationSeconds("CAPACITY_DURATION_SECONDS",60,1,1800);if err!=nil{return err}
	concurrency,err:=envInt("CAPACITY_CONCURRENCY",16,1,512);if err!=nil{return err}
	targetQPS,err:=envFloat("CAPACITY_TARGET_QPS",100,0,1_000_000);if err!=nil{return err}
	manticoreBytes,err:=envInt64Required("CAPACITY_MANTICORE_BYTES",0);if err!=nil{return fmt.Errorf("CAPACITY_MANTICORE_BYTES must contain a measured Manticore data size: %w",err)}
	searchURLs,err:=searchURLsFromFile(base,os.Getenv("CAPACITY_SEARCH_QUERIES"));if err!=nil{return err}
	geoURLs,err:=endpointURLsFromFile(base,os.Getenv("CAPACITY_GEO_URLS"),"/api/geo/");if err!=nil{return err}
	addressURLs,err:=endpointURLsFromFile(base,os.Getenv("CAPACITY_ADDRESS_URLS"),"/api/address/");if err!=nil{return err}
	config:=map[string]any{"base_url":base,"duration_seconds":duration.Seconds(),"concurrency":concurrency,"target_qps":targetQPS,"search_cases":len(searchURLs),"geo_cases":len(geoURLs),"address_cases":len(addressURLs)}
	runID,err:=repo.StartRun(ctx,label,mode,10_000_000,config);if err!=nil{return err}
	failed:=true;defer func(){if failed{_ = repo.FailRun(context.Background(),runID,"benchmark command terminated before immutable snapshot")}}()

	dbBefore,err:=repo.CollectDatabase(ctx);if err!=nil{return err}
	if dbBefore.Corpus.IndexedDocuments<=0{return errors.New("capacity benchmark requires a non-empty indexed corpus")}
	if mode=="ISOLATED_1M"&&dbBefore.Corpus.IndexedDocuments<1_000_000{return fmt.Errorf("ISOLATED_1M requires at least 1,000,000 indexed documents, got %d",dbBefore.Corpus.IndexedDocuments)}
	resourceMark:=capacity.MarkResources()
	search,err:=capacity.RunHTTPBenchmark(ctx,capacity.HTTPBenchmarkConfig{URLs:searchURLs,Duration:duration,Concurrency:concurrency,RequestTimeout:3*time.Second});if err!=nil{return err}
	geo,err:=capacity.RunHTTPBenchmark(ctx,capacity.HTTPBenchmarkConfig{URLs:geoURLs,Duration:duration,Concurrency:concurrency,RequestTimeout:3*time.Second});if err!=nil{return err}
	address,err:=capacity.RunHTTPBenchmark(ctx,capacity.HTTPBenchmarkConfig{URLs:addressURLs,Duration:duration,Concurrency:concurrency,RequestTimeout:3*time.Second});if err!=nil{return err}
	resources:=capacity.MeasureResources(resourceMark,envDefault("CAPACITY_DISK_PATH","/"))
	dbAfter,err:=repo.CollectDatabase(ctx);if err!=nil{return err}
	storage:=capacity.StorageSnapshot{DatabaseBytes:dbAfter.DatabaseBytes,ManticoreBytes:manticoreBytes}
	projection,err:=capacity.ProjectStorage(dbAfter.Corpus.IndexedDocuments,10_000_000,storage);if err!=nil{return err}
	signals:=capacity.Signals{Search:search,GEO:geo,Address:address,CPUPercent:resources.CPUPercent,RAMPercent:resources.RAMPercent,DiskPercent:resources.DiskPercent,CrawlReady:dbAfter.Queues.CrawlReady,OutboxReady:dbAfter.Queues.OutboxReady,CrawlPerSecond:dbAfter.Throughput.CrawlPerSecond,IndexPerSecond:dbAfter.Throughput.IndexPerSecond,TargetQPS:targetQPS,ProjectedBytes:projection.ProjectedTotalBytes,AvailableDiskBytes:resources.DiskAvailableBytes}
	bottlenecks:=capacity.Classify(signals)
	resourceMap:=map[string]any{"cpu_percent":resources.CPUPercent,"ram_percent":resources.RAMPercent,"disk_percent":resources.DiskPercent,"memory_current_bytes":resources.MemoryCurrentBytes,"memory_limit_bytes":resources.MemoryLimitBytes,"disk_total_bytes":resources.DiskTotalBytes,"disk_available_bytes":resources.DiskAvailableBytes,"cpu_cores":resources.CPUCores,"probe_warnings":resources.ProbeWarnings}
	queueMap:=map[string]any{"before":dbBefore.Queues,"after":dbAfter.Queues,"throughput":dbAfter.Throughput,"corpus":dbAfter.Corpus}
	snapshotID,err:=repo.CompleteRun(ctx,capacity.FinalSnapshot{RunID:runID,MeasuredDocuments:dbAfter.Corpus.IndexedDocuments,MeasuredAt:time.Now().UTC(),Workload:map[string]capacity.WorkloadMetrics{"search":search,"geo":geo,"address":address},Resources:resourceMap,Queues:queueMap,Storage:storage,Projection:projection,Bottlenecks:bottlenecks});if err!=nil{return err}
	failed=false
	out:=map[string]any{"run_id":runID,"snapshot_id":snapshotID,"mode":mode,"workload":map[string]capacity.WorkloadMetrics{"search":search,"geo":geo,"address":address},"database":dbAfter,"resources":resources,"projection_10m":projection,"bottlenecks":bottlenecks};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");return enc.Encode(out)
}

func searchURLsFromFile(base,path string)([]string,error){lines,err:=boundedLines(path);if err!=nil{return nil,fmt.Errorf("search queries: %w",err)};out:=make([]string,0,len(lines));for _,q:=range lines{v:=url.Values{};v.Set("q",q);v.Set("limit","10");out=append(out,base+"/api/search?"+v.Encode())};return out,nil}
func endpointURLsFromFile(base,path,prefix string)([]string,error){lines,err:=boundedLines(path);if err!=nil{return nil,err};out:=make([]string,0,len(lines));for _,p:=range lines{if !strings.HasPrefix(p,prefix)||strings.HasPrefix(p,"//"){return nil,fmt.Errorf("benchmark path must start with %s",prefix)};u,err:=url.Parse(p);if err!=nil||u.IsAbs()||u.Host!=""{return nil,errors.New("benchmark endpoint must be a relative API path")};out=append(out,base+p)};return out,nil}
func boundedLines(path string)([]string,error){path=strings.TrimSpace(path);if path==""{return nil,errors.New("benchmark file path is required")};f,err:=os.Open(path);if err!=nil{return nil,err};defer f.Close();s:=bufio.NewScanner(f);s.Buffer(make([]byte,1024),4096);out:=make([]string,0,256);for s.Scan(){line:=strings.TrimSpace(s.Text());if line==""||strings.HasPrefix(line,"#"){continue};if len([]rune(line))>512{return nil,errors.New("benchmark line exceeds 512 characters")};out=append(out,line);if len(out)>10000{return nil,errors.New("benchmark file exceeds 10000 cases")}};if err:=s.Err();err!=nil{return nil,err};if len(out)==0{return nil,errors.New("benchmark file has no cases")};return out,nil}
func envDefault(key,def string)string{if v:=strings.TrimSpace(os.Getenv(key));v!=""{return v};return def}
func envInt(key string,def,min,max int)(int,error){raw:=envDefault(key,strconv.Itoa(def));v,err:=strconv.Atoi(raw);if err!=nil||v<min||v>max{return 0,fmt.Errorf("invalid %s",key)};return v,nil}
func envDurationSeconds(key string,def,min,max int)(time.Duration,error){v,err:=envInt(key,def,min,max);return time.Duration(v)*time.Second,err}
func envFloat(key string,def,min,max float64)(float64,error){raw:=envDefault(key,strconv.FormatFloat(def,'f',-1,64));v,err:=strconv.ParseFloat(raw,64);if err!=nil||v<min||v>max{return 0,fmt.Errorf("invalid %s",key)};return v,nil}
func envInt64Required(key string,min int64)(int64,error){raw:=strings.TrimSpace(os.Getenv(key));if raw==""{return 0,errors.New("value is required")};v,err:=strconv.ParseInt(raw,10,64);if err!=nil||v<min{return 0,errors.New("invalid integer")};return v,nil}
