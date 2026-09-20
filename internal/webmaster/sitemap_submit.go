package webmaster

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) QueueSitemapSubmission(ctx context.Context,userID,siteID int64,sitemapURL string)(int64,error){
	if r==nil || r.db==nil{return 0,errors.New("webmaster repository is not initialized")}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,err};defer func(){_=tx.Rollback(ctx)}()
	var id int64
	err=tx.QueryRow(ctx,`
INSERT INTO webmaster_sitemaps(site_id,sitemap_url,status,available_at)
SELECT s.site_id,$3,'SUBMITTED',now() FROM webmaster_sites s
WHERE s.site_id=$1 AND s.user_id=$2 AND s.status='VERIFIED'
ON CONFLICT(site_id,sitemap_url) DO UPDATE SET
  status=CASE WHEN webmaster_sitemaps.status='LEASED' THEN 'LEASED' ELSE 'SUBMITTED' END,
  available_at=CASE WHEN webmaster_sitemaps.status='LEASED' THEN webmaster_sitemaps.available_at ELSE now() END,
  last_error=CASE WHEN webmaster_sitemaps.status='LEASED' THEN webmaster_sitemaps.last_error ELSE NULL END,
  submitted_at=now(),updated_at=now()
RETURNING sitemap_id`,siteID,userID,sitemapURL).Scan(&id)
	if errors.Is(err,pgx.ErrNoRows){return 0,ErrNotVerified}
	if err!=nil{return 0,fmt.Errorf("queue sitemap submission: %w",err)}
	if err:=auditTx(ctx,tx,userID,"WEBMASTER_SITEMAP_SUBMIT","WEBMASTER_SITE",siteID,map[string]string{"url":sitemapURL});err!=nil{return 0,err}
	if err:=tx.Commit(ctx);err!=nil{return 0,err}
	return id,nil
}
