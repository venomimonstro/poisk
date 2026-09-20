package webmaster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type SitemapTask struct {
	ID          int64
	SiteID      int64
	DomainID    int64
	Host        string
	URL         string
	Depth       int
	Attempts    int
	MaxAttempts int
	WorkerID    string
}

func (r *Repository) LeaseSitemaps(ctx context.Context, workerID string, batchSize, leaseSeconds int) ([]SitemapTask,error) {
	if r==nil || r.db==nil { return nil,errors.New("webmaster repository is not initialized") }
	if workerID=="" || batchSize<=0 || batchSize>100 || leaseSeconds<=0 { return nil,ErrInvalidInput }
	const q=`
WITH picked AS (
    SELECT ws.sitemap_id
    FROM webmaster_sitemaps ws
    JOIN webmaster_sites s ON s.site_id=ws.site_id
    WHERE ws.status IN ('SUBMITTED','RETRY')
      AND ws.available_at<=now()
      AND ws.attempts<ws.max_attempts
      AND s.status='VERIFIED'
    ORDER BY ws.available_at,ws.sitemap_id
    FOR UPDATE OF ws SKIP LOCKED
    LIMIT $1
)
UPDATE webmaster_sitemaps ws
SET status='LEASED',attempts=attempts+1,
    lease_until=now()+make_interval(secs=>$2::int),worker_id=$3,updated_at=now()
FROM picked,webmaster_sites s
WHERE ws.sitemap_id=picked.sitemap_id AND s.site_id=ws.site_id
RETURNING ws.sitemap_id,ws.site_id,s.domain_id,s.host,ws.sitemap_url,ws.depth,ws.attempts,ws.max_attempts,ws.worker_id`
	rows,err:=r.db.Query(ctx,q,batchSize,leaseSeconds,workerID)
	if err!=nil { return nil,fmt.Errorf("lease webmaster sitemaps: %w",err) }
	defer rows.Close()
	out:=make([]SitemapTask,0,batchSize)
	for rows.Next(){
		var task SitemapTask
		if err:=rows.Scan(&task.ID,&task.SiteID,&task.DomainID,&task.Host,&task.URL,&task.Depth,&task.Attempts,&task.MaxAttempts,&task.WorkerID);err!=nil{return nil,err}
		out=append(out,task)
	}
	return out,rows.Err()
}

func (r *Repository) RequeueExpiredSitemaps(ctx context.Context)(int64,error){
	tag,err:=r.db.Exec(ctx,`
UPDATE webmaster_sitemaps
SET status=CASE WHEN attempts>=max_attempts THEN 'FAILED' ELSE 'RETRY' END,
    available_at=CASE WHEN attempts>=max_attempts THEN available_at ELSE now() END,
    lease_until=NULL,worker_id=NULL,
    last_error=COALESCE(last_error,'lease_expired'),updated_at=now()
WHERE status='LEASED' AND lease_until<now()`)
	if err!=nil{return 0,fmt.Errorf("requeue expired webmaster sitemaps: %w",err)}
	return tag.RowsAffected(),nil
}

func (r *Repository) CompleteSitemap(ctx context.Context,task SitemapTask) error {
	tag,err:=r.db.Exec(ctx,`
UPDATE webmaster_sitemaps SET status='FETCHED',lease_until=NULL,worker_id=NULL,last_error=NULL,updated_at=now()
WHERE sitemap_id=$1 AND status='LEASED' AND worker_id=$2`,task.ID,task.WorkerID)
	if err!=nil{return err}
	if tag.RowsAffected()!=1{return errors.New("sitemap lease ownership lost")}
	return nil
}

func (r *Repository) RetrySitemap(ctx context.Context,task SitemapTask,cause error,retryAfter time.Duration) error {
	if retryAfter<0{retryAfter=0}
	message:="sitemap processing failed"; if cause!=nil{message=cause.Error()}; if len(message)>2048{message=message[:2048]}
	tag,err:=r.db.Exec(ctx,`
UPDATE webmaster_sitemaps
SET status=CASE WHEN attempts>=max_attempts THEN 'FAILED' ELSE 'RETRY' END,
    available_at=CASE WHEN attempts>=max_attempts THEN available_at ELSE now()+$3::interval END,
    lease_until=NULL,worker_id=NULL,last_error=$4,updated_at=now()
WHERE sitemap_id=$1 AND status='LEASED' AND worker_id=$2`,task.ID,task.WorkerID,retryAfter.String(),message)
	if err!=nil{return err}
	if tag.RowsAffected()!=1{return errors.New("sitemap lease ownership lost")}
	return nil
}

func (r *Repository) AddChildSitemaps(ctx context.Context,task SitemapTask,urls []string) error {
	if len(urls)==0{return nil}
	if task.Depth>=8{return ErrInvalidInput}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{}); if err!=nil{return err}; defer func(){_=tx.Rollback(ctx)}()
	for _,raw:=range urls{
		_,err:=tx.Exec(ctx,`
INSERT INTO webmaster_sitemaps(site_id,parent_sitemap_id,sitemap_url,depth,status,available_at)
VALUES($1,$2,$3,$4,'SUBMITTED',now())
ON CONFLICT(site_id,sitemap_url) DO UPDATE SET
  status=CASE WHEN webmaster_sitemaps.status='DELETED' THEN 'SUBMITTED' ELSE webmaster_sitemaps.status END,
  updated_at=now()`,task.SiteID,task.ID,raw,task.Depth+1)
		if err!=nil{return fmt.Errorf("enqueue child sitemap: %w",err)}
	}
	return tx.Commit(ctx)
}

func (r *Repository) AddSitemapURLs(ctx context.Context,task SitemapTask,urls []string) error {
	if len(urls)==0{return nil}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{}); if err!=nil{return err}; defer func(){_=tx.Rollback(ctx)}()
	for _,raw:=range urls{
		var urlID,version,domainID int64
		err:=tx.QueryRow(ctx,`
INSERT INTO urls(domain_id,normalized_url,crawl_status,index_status,next_crawl_at)
VALUES($1,$2,'DISCOVERED','NOT_INDEXED',now())
ON CONFLICT(normalized_url) DO UPDATE SET next_crawl_at=LEAST(COALESCE(urls.next_crawl_at,now()),now()),updated_at=now()
RETURNING url_id,version,domain_id`,task.DomainID,raw).Scan(&urlID,&version,&domainID)
		if err!=nil{return fmt.Errorf("ensure sitemap URL: %w",err)}
		if domainID!=task.DomainID{return ErrNotFound}
		if _,err:=tx.Exec(ctx,`
INSERT INTO crawl_queue(url_id,domain_id,generation,priority,status,available_at)
VALUES($1,$2,$3,80,'READY',now())
ON CONFLICT(url_id,generation) WHERE status IN ('READY','LEASED','RETRY')
DO UPDATE SET priority=GREATEST(crawl_queue.priority,EXCLUDED.priority),available_at=LEAST(crawl_queue.available_at,EXCLUDED.available_at),updated_at=now()`,urlID,task.DomainID,version);err!=nil{return fmt.Errorf("enqueue sitemap URL: %w",err)}
	}
	return tx.Commit(ctx)
}
