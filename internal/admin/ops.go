package admin

import (
	"context"
	"errors"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

type QueueCounts struct {
	Ready int64 `json:"ready"`
	Leased int64 `json:"leased"`
	Retry int64 `json:"retry"`
	Dead int64 `json:"dead"`
}
type OutboxCounts struct {
	EntityType string `json:"entity_type"`
	QueueCounts
}
type ImportCounts struct {
	Organizations int64 `json:"organizations_active"`
	Addresses int64 `json:"addresses_active"`
	WebmasterSitemaps int64 `json:"webmaster_sitemaps_active"`
}
type RuntimeStats struct {
	Goroutines int `json:"goroutines"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes uint64 `json:"heap_sys_bytes"`
}
type OpsSnapshot struct {
	Crawl QueueCounts `json:"crawl"`
	Outbox []OutboxCounts `json:"outbox"`
	Imports ImportCounts `json:"imports"`
	Runtime RuntimeStats `json:"runtime"`
}

type OpsRepository struct{DB *pgxpool.Pool}
func (o OpsRepository) Snapshot(ctx context.Context)(OpsSnapshot,error){
	if o.DB==nil{return OpsSnapshot{},errors.New("ops repository is not initialized")}
	var out OpsSnapshot
	if err:=o.DB.QueryRow(ctx,`SELECT count(*) FILTER(WHERE status='READY'),count(*) FILTER(WHERE status='LEASED'),count(*) FILTER(WHERE status='RETRY'),count(*) FILTER(WHERE status='DEAD') FROM crawl_queue`).Scan(&out.Crawl.Ready,&out.Crawl.Leased,&out.Crawl.Retry,&out.Crawl.Dead);err!=nil{return OpsSnapshot{},err}
	rows,err:=o.DB.Query(ctx,`SELECT entity_type,count(*) FILTER(WHERE status='READY'),count(*) FILTER(WHERE status='LEASED'),count(*) FILTER(WHERE status='RETRY'),count(*) FILTER(WHERE status='DEAD') FROM index_outbox GROUP BY entity_type ORDER BY entity_type`);if err!=nil{return OpsSnapshot{},err};defer rows.Close()
	for rows.Next(){var item OutboxCounts;if err:=rows.Scan(&item.EntityType,&item.Ready,&item.Leased,&item.Retry,&item.Dead);err!=nil{return OpsSnapshot{},err};out.Outbox=append(out.Outbox,item)};if err:=rows.Err();err!=nil{return OpsSnapshot{},err}
	if err:=o.DB.QueryRow(ctx,`SELECT
 (SELECT count(*) FROM organization_import_batches WHERE status IN ('STAGING','PLANNING','PLANNED','APPLYING')),
 (SELECT count(*) FROM address_import_batches WHERE status IN ('STAGING','RESOLVING','APPLYING')),
 (SELECT count(*) FROM webmaster_sitemaps WHERE status IN ('SUBMITTED','LEASED','RETRY'))`).Scan(&out.Imports.Organizations,&out.Imports.Addresses,&out.Imports.WebmasterSitemaps);err!=nil{return OpsSnapshot{},err}
	var mem runtime.MemStats;runtime.ReadMemStats(&mem);out.Runtime=RuntimeStats{Goroutines:runtime.NumGoroutine(),HeapAllocBytes:mem.HeapAlloc,HeapSysBytes:mem.HeapSys}
	return out,nil
}
