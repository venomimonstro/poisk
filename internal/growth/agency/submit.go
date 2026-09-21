package agency

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	urlnorm "github.com/venomimonstro/poisk/internal/crawler/urlnorm"
)

type managedSite struct{DomainID int64;Host string}

func (r *Repository) managedSite(ctx context.Context,tx pgx.Tx,userID,agencyID,siteID int64)(managedSite,error){
	var out managedSite
	err:=tx.QueryRow(ctx,`SELECT s.domain_id,s.host
FROM agency_members m
JOIN agencies g ON g.agency_id=m.agency_id
JOIN agency_site_access a ON a.agency_id=g.agency_id
JOIN webmaster_sites s ON s.site_id=a.site_id
JOIN domains d ON d.domain_id=s.domain_id
WHERE m.agency_id=$1 AND m.user_id=$2 AND m.status='ACTIVE' AND m.role IN ('OWNER','MANAGER')
  AND g.status='ACTIVE' AND a.site_id=$3 AND a.status='ACTIVE' AND a.permission='MANAGE'
  AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK'`,agencyID,userID,siteID).Scan(&out.DomainID,&out.Host)
	if errors.Is(err,pgx.ErrNoRows){return managedSite{},ErrForbidden};if err!=nil{return managedSite{},err};return out,nil
}

func normalizeManagedURL(raw,host string)(string,error){
	normalized,err:=urlnorm.Normalize(strings.TrimSpace(raw));if err!=nil{return "",ErrInvalid}
	u,err:=url.Parse(normalized);if err!=nil||u.User!=nil||!strings.EqualFold(strings.TrimSuffix(u.Hostname(),"."),strings.TrimSuffix(host,".")){return "",ErrForbidden}
	return normalized,nil
}

func (r *Repository) SubmitSitemap(ctx context.Context,userID,agencyID,siteID int64,rawURL string)(int64,error){
	if r==nil||r.db==nil||userID<=0||agencyID<=0||siteID<=0{return 0,ErrInvalid}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,err};defer func(){_=tx.Rollback(ctx)}()
	site,err:=r.managedSite(ctx,tx,userID,agencyID,siteID);if err!=nil{return 0,err};normalized,err:=normalizeManagedURL(rawURL,site.Host);if err!=nil{return 0,err}
	var id int64
	err=tx.QueryRow(ctx,`INSERT INTO webmaster_sitemaps(site_id,sitemap_url,status,available_at)
VALUES($1,$2,'SUBMITTED',now())
ON CONFLICT(site_id,sitemap_url) DO UPDATE SET
 status=CASE WHEN webmaster_sitemaps.status='LEASED' THEN 'LEASED' ELSE 'SUBMITTED' END,
 available_at=CASE WHEN webmaster_sitemaps.status='LEASED' THEN webmaster_sitemaps.available_at ELSE now() END,
 last_error=CASE WHEN webmaster_sitemaps.status='LEASED' THEN webmaster_sitemaps.last_error ELSE NULL END,
 submitted_at=now(),updated_at=now()
RETURNING sitemap_id`,siteID,normalized).Scan(&id);if err!=nil{return 0,fmt.Errorf("agency sitemap submit: %w",err)}
	if _,err=tx.Exec(ctx,`INSERT INTO agency_audit_events(agency_id,actor_user_id,site_id,action,details) VALUES($1,$2,$3,'SITEMAP_SUBMIT',jsonb_build_object('url',$4))`,agencyID,userID,siteID,normalized);err!=nil{return 0,err}
	if err=tx.Commit(ctx);err!=nil{return 0,err};return id,nil
}

func (r *Repository) SubmitURL(ctx context.Context,userID,agencyID,siteID int64,rawURL,operation string)(int64,error){
	operation=strings.ToUpper(strings.TrimSpace(operation));if r==nil||r.db==nil||userID<=0||agencyID<=0||siteID<=0||(operation!="SUBMIT"&&operation!="REINDEX"&&operation!="DELETE"){return 0,ErrInvalid}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,err};defer func(){_=tx.Rollback(ctx)}()
	site,err:=r.managedSite(ctx,tx,userID,agencyID,siteID);if err!=nil{return 0,err};normalized,err:=normalizeManagedURL(rawURL,site.Host);if err!=nil{return 0,err}
	var urlID,version,domainID int64
	switch operation{
	case "SUBMIT","REINDEX":
		err=tx.QueryRow(ctx,`INSERT INTO urls(domain_id,normalized_url,crawl_status,index_status,next_crawl_at)
VALUES($1,$2,'DISCOVERED','NOT_INDEXED',now())
ON CONFLICT(normalized_url) DO UPDATE SET next_crawl_at=now(),index_status=CASE WHEN urls.index_status='DELETED' THEN 'NOT_INDEXED' ELSE urls.index_status END,crawl_status=CASE WHEN urls.crawl_status='BLOCKED' THEN 'BLOCKED' ELSE 'DISCOVERED' END,updated_at=now()
RETURNING url_id,version,domain_id`,site.DomainID,normalized).Scan(&urlID,&version,&domainID);if err!=nil{return 0,err};if domainID!=site.DomainID{return 0,ErrForbidden}
		if _,err=tx.Exec(ctx,`INSERT INTO crawl_queue(url_id,domain_id,generation,priority,status,available_at)
SELECT $1,$2,$3,90,'READY',now() WHERE EXISTS(SELECT 1 FROM urls WHERE url_id=$1 AND crawl_status<>'BLOCKED') ON CONFLICT DO NOTHING`,urlID,site.DomainID,version);err!=nil{return 0,err}
	case "DELETE":
		err=tx.QueryRow(ctx,`UPDATE urls SET version=version+1,index_status='DELETED',updated_at=now() WHERE domain_id=$1 AND normalized_url=$2 RETURNING url_id,version`,site.DomainID,normalized).Scan(&urlID,&version);if errors.Is(err,pgx.ErrNoRows){return 0,ErrNotFound};if err!=nil{return 0,err}
		if _,err=tx.Exec(ctx,`INSERT INTO index_outbox(entity_type,entity_id,entity_version,operation,available_at) VALUES('WEB_DOCUMENT',$1,$2,'DELETE',now()) ON CONFLICT(entity_type,entity_id,entity_version) DO NOTHING`,urlID,version);err!=nil{return 0,err}
	}
	var requestID int64
	err=tx.QueryRow(ctx,`INSERT INTO webmaster_url_requests(site_id,url_id,normalized_url,operation,status) VALUES($1,$2,$3,$4,'QUEUED') RETURNING request_id`,siteID,urlID,normalized,operation).Scan(&requestID);if err!=nil{return 0,fmt.Errorf("agency URL request: %w",err)}
	if _,err=tx.Exec(ctx,`INSERT INTO agency_audit_events(agency_id,actor_user_id,site_id,action,details) VALUES($1,$2,$3,'URL_SUBMIT',jsonb_build_object('url',$4,'operation',$5))`,agencyID,userID,siteID,normalized,operation);err!=nil{return 0,err}
	if err=tx.Commit(ctx);err!=nil{return 0,err};return requestID,nil
}
