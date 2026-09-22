package capacity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type CorpusSnapshot struct{
	URLs int64 `json:"urls"`
	IndexedDocuments int64 `json:"indexed_documents"`
	Organizations int64 `json:"organizations"`
	Addresses int64 `json:"addresses"`
	DataHubPublished int64 `json:"datahub_published"`
}

type QueueSnapshot struct{
	CrawlReady int64 `json:"crawl_ready"`
	CrawlLeased int64 `json:"crawl_leased"`
	CrawlDead int64 `json:"crawl_dead"`
	OutboxReady int64 `json:"outbox_ready"`
	OutboxLeased int64 `json:"outbox_leased"`
	OutboxDead int64 `json:"outbox_dead"`
}

type ThroughputSnapshot struct{
	CrawlPerSecond float64 `json:"crawl_per_second"`
	ExtractPerSecond float64 `json:"extract_per_second"`
	IndexPerSecond float64 `json:"index_per_second"`
	DataHubChangesPerSecond float64 `json:"datahub_changes_per_second"`
	WindowSeconds int `json:"window_seconds"`
}

type DatabaseSnapshot struct{
	Corpus CorpusSnapshot `json:"corpus"`
	Queues QueueSnapshot `json:"queues"`
	Throughput ThroughputSnapshot `json:"throughput"`
	DatabaseBytes int64 `json:"database_bytes"`
	CapturedAt time.Time `json:"captured_at"`
}

type FinalSnapshot struct{
	RunID int64 `json:"run_id"`
	MeasuredDocuments int64 `json:"measured_documents"`
	MeasuredAt time.Time `json:"measured_at"`
	Workload map[string]WorkloadMetrics `json:"workload"`
	Resources map[string]any `json:"resources"`
	Queues map[string]any `json:"queues"`
	Storage StorageSnapshot `json:"storage"`
	Projection Projection `json:"projection_10m"`
	Bottlenecks []Bottleneck `json:"bottlenecks"`
}

func (r *Repository) StartRun(ctx context.Context,label,mode string,targetDocuments int64,config any)(int64,error){
	label=strings.TrimSpace(label);mode=strings.ToUpper(strings.TrimSpace(mode));if r==nil||r.db==nil||label==""||len(label)>128||(mode!="LIVE_READONLY"&&mode!="ISOLATED_1M")||targetDocuments<=0||targetDocuments>100_000_000{return 0,ErrInvalid}
	raw,err:=json.Marshal(config);if err!=nil{return 0,err};var id int64
	err=r.db.QueryRow(ctx,`INSERT INTO capacity_benchmark_runs(label,mode,target_documents,config) VALUES($1,$2,$3,$4::jsonb) RETURNING run_id`,label,mode,targetDocuments,string(raw)).Scan(&id);return id,err
}

func (r *Repository) FailRun(ctx context.Context,runID int64,reason string)error{
	reason=strings.TrimSpace(reason);if r==nil||r.db==nil||runID<=0||reason==""{return ErrInvalid};if len(reason)>4000{reason=reason[:4000]}
	tag,err:=r.db.Exec(ctx,`UPDATE capacity_benchmark_runs SET status='FAILED',completed_at=now(),last_error=$2 WHERE run_id=$1 AND status='RUNNING'`,runID,reason);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrInvalid};return nil
}

func (r *Repository) CollectDatabase(ctx context.Context)(DatabaseSnapshot,error){
	if r==nil||r.db==nil{return DatabaseSnapshot{},ErrInvalid};out:=DatabaseSnapshot{CapturedAt:time.Now().UTC(),Throughput:ThroughputSnapshot{WindowSeconds:3600}}
	if err:=r.db.QueryRow(ctx,`SELECT
 (SELECT count(*) FROM urls),
 (SELECT count(*) FROM urls WHERE index_status='INDEXED'),
 (SELECT count(*) FROM organizations WHERE status='ACTIVE'),
 (SELECT count(*) FROM addresses WHERE status='ACTIVE'),
 (SELECT count(*) FROM datahub_pages WHERE state='PUBLISHED'),
 pg_database_size(current_database())`).Scan(&out.Corpus.URLs,&out.Corpus.IndexedDocuments,&out.Corpus.Organizations,&out.Corpus.Addresses,&out.Corpus.DataHubPublished,&out.DatabaseBytes);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT
 count(*) FILTER(WHERE status IN ('READY','RETRY')),
 count(*) FILTER(WHERE status='LEASED'),
 count(*) FILTER(WHERE status='DEAD')
 FROM crawl_queue`).Scan(&out.Queues.CrawlReady,&out.Queues.CrawlLeased,&out.Queues.CrawlDead);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT
 count(*) FILTER(WHERE status IN ('READY','RETRY')),
 count(*) FILTER(WHERE status='LEASED'),
 count(*) FILTER(WHERE status='DEAD')
 FROM index_outbox`).Scan(&out.Queues.OutboxReady,&out.Queues.OutboxLeased,&out.Queues.OutboxDead);err!=nil{return out,err}
	var crawled,extracted,indexed,hub int64
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM crawl_history WHERE completed_at>=now()-interval '1 hour'`).Scan(&crawled);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM document_content WHERE created_at>=now()-interval '1 hour'`).Scan(&extracted);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM index_outbox WHERE processed_at>=now()-interval '1 hour' AND status='PROCESSED'`).Scan(&indexed);err!=nil{return out,err}
	if err:=r.db.QueryRow(ctx,`SELECT count(*) FROM datahub_publication_events WHERE created_at>=now()-interval '1 hour'`).Scan(&hub);err!=nil{return out,err}
	out.Throughput.CrawlPerSecond=float64(crawled)/3600;out.Throughput.ExtractPerSecond=float64(extracted)/3600;out.Throughput.IndexPerSecond=float64(indexed)/3600;out.Throughput.DataHubChangesPerSecond=float64(hub)/3600
	return out,nil
}

func (r *Repository) CompleteRun(ctx context.Context,s FinalSnapshot)(int64,error){
	if r==nil||r.db==nil||s.RunID<=0||s.MeasuredDocuments<0{return 0,ErrInvalid};if s.MeasuredAt.IsZero(){s.MeasuredAt=time.Now().UTC()}
	workload,err:=json.Marshal(s.Workload);if err!=nil{return 0,err};resources,err:=json.Marshal(s.Resources);if err!=nil{return 0,err};queues,err:=json.Marshal(s.Queues);if err!=nil{return 0,err};storage,err:=json.Marshal(s.Storage);if err!=nil{return 0,err};projection,err:=json.Marshal(s.Projection);if err!=nil{return 0,err};bottlenecks,err:=json.Marshal(s.Bottlenecks);if err!=nil{return 0,err}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,err};defer func(){_=tx.Rollback(ctx)}();var status string
	if err=tx.QueryRow(ctx,`SELECT status FROM capacity_benchmark_runs WHERE run_id=$1 FOR UPDATE`,s.RunID).Scan(&status);errors.Is(err,pgx.ErrNoRows){return 0,ErrInvalid};if err!=nil{return 0,err};if status!="RUNNING"{return 0,ErrInvalid}
	var snapshotID int64
	err=tx.QueryRow(ctx,`INSERT INTO capacity_snapshots(run_id,measured_documents,measured_at,workload,resources,queues,storage,projection_10m,bottlenecks)
VALUES($1,$2,$3,$4::jsonb,$5::jsonb,$6::jsonb,$7::jsonb,$8::jsonb,$9::jsonb) RETURNING snapshot_id`,s.RunID,s.MeasuredDocuments,s.MeasuredAt,string(workload),string(resources),string(queues),string(storage),string(projection),string(bottlenecks)).Scan(&snapshotID);if err!=nil{return 0,err}
	if _,err=tx.Exec(ctx,`UPDATE capacity_benchmark_runs SET status='COMPLETED',completed_at=now() WHERE run_id=$1`,s.RunID);err!=nil{return 0,err};if err=tx.Commit(ctx);err!=nil{return 0,err};return snapshotID,nil
}
