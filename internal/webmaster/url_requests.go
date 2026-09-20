package webmaster

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// QueueURLRequest turns a verified Webmaster request into canonical work. It
// never writes to Manticore directly: SUBMIT/REINDEX enqueue crawl work, while
// DELETE advances the canonical URL version and emits a transactional outbox
// event for the indexer.
func (r *Repository) QueueURLRequest(ctx context.Context, userID, siteID int64, normalizedURL, operation string) (int64, error) {
	if r == nil || r.db == nil { return 0, errors.New("webmaster repository is not initialized") }
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return 0, err }
	defer func(){ _ = tx.Rollback(ctx) }()

	var siteDomainID int64
	err = tx.QueryRow(ctx, `SELECT domain_id FROM webmaster_sites WHERE site_id=$1 AND user_id=$2 AND status='VERIFIED' FOR SHARE`, siteID, userID).Scan(&siteDomainID)
	if errors.Is(err, pgx.ErrNoRows) { return 0, ErrNotVerified }
	if err != nil { return 0, fmt.Errorf("load verified webmaster site: %w", err) }

	var urlID, version, urlDomainID int64
	switch operation {
	case "SUBMIT", "REINDEX":
		err = tx.QueryRow(ctx, `
INSERT INTO urls(domain_id,normalized_url,crawl_status,index_status,next_crawl_at)
VALUES($1,$2,'DISCOVERED','NOT_INDEXED',now())
ON CONFLICT(normalized_url) DO UPDATE SET next_crawl_at=now(), updated_at=now()
RETURNING url_id,version,domain_id`, siteDomainID, normalizedURL).Scan(&urlID,&version,&urlDomainID)
		if err != nil { return 0, fmt.Errorf("ensure webmaster URL: %w",err) }
		if urlDomainID != siteDomainID { return 0, ErrNotFound }
		if _,err=tx.Exec(ctx, `
INSERT INTO crawl_queue(url_id,domain_id,generation,priority,status,available_at)
VALUES($1,$2,$3,100,'READY',now())
ON CONFLICT DO NOTHING`,urlID,siteDomainID,version); err!=nil { return 0,fmt.Errorf("enqueue webmaster crawl: %w",err) }
	case "DELETE":
		err = tx.QueryRow(ctx, `
UPDATE urls
SET version=version+1,index_status='DELETED',updated_at=now()
WHERE domain_id=$1 AND normalized_url=$2
RETURNING url_id,version`,siteDomainID,normalizedURL).Scan(&urlID,&version)
		if errors.Is(err,pgx.ErrNoRows) { return 0,ErrNotFound }
		if err!=nil { return 0,fmt.Errorf("mark webmaster URL deleted: %w",err) }
		if _,err=tx.Exec(ctx, `
INSERT INTO index_outbox(entity_type,entity_id,entity_version,operation,available_at)
VALUES('WEB_DOCUMENT',$1,$2,'DELETE',now())
ON CONFLICT(entity_type,entity_id,entity_version,operation) DO NOTHING`,urlID,version); err!=nil { return 0,fmt.Errorf("enqueue webmaster delete: %w",err) }
	default:
		return 0, errors.New("invalid webmaster URL operation")
	}

	var requestID int64
	err=tx.QueryRow(ctx, `
INSERT INTO webmaster_url_requests(site_id,url_id,normalized_url,operation,status)
VALUES($1,$2,$3,$4,'QUEUED')
RETURNING request_id`,siteID,urlID,normalizedURL,operation).Scan(&requestID)
	if isUniqueViolation(err) { return 0,ErrConflict }
	if err!=nil { return 0,fmt.Errorf("record webmaster URL request: %w",err) }
	if err:=auditTx(ctx,tx,userID,"WEBMASTER_URL_"+operation,"WEBMASTER_SITE",siteID,map[string]string{"url":normalizedURL}); err!=nil { return 0,err }
	if err:=tx.Commit(ctx); err!=nil { return 0,err }
	return requestID,nil
}
