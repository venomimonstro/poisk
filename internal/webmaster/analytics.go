package webmaster

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/venomimonstro/poisk/internal/crawler/urlnorm"
)

func (r *Repository) RecordSearchImpressions(ctx context.Context, hosts []string) error {
	return r.recordHostMetric(ctx,hosts,"impressions")
}

func (r *Repository) RecordAnswerCitations(ctx context.Context, hosts []string) error {
	return r.recordHostMetric(ctx,hosts,"answer_citations")
}

func (r *Repository) RecordClick(ctx context.Context, rawURL string) error {
	if r==nil || r.db==nil{return errors.New("webmaster repository is not initialized")}
	normalized,err:=urlnorm.Normalize(strings.TrimSpace(rawURL))
	if err!=nil{return ErrInvalidInput}
	_,err=r.db.Exec(ctx,`
WITH matched AS (
    SELECT s.site_id
    FROM urls u
    JOIN webmaster_sites s ON s.domain_id=u.domain_id
    WHERE u.normalized_url=$1
      AND u.index_status='INDEXED'
      AND s.status='VERIFIED'
)
INSERT INTO webmaster_metrics_daily(site_id,day,clicks)
SELECT site_id,current_date,1 FROM matched
ON CONFLICT(site_id,day) DO UPDATE
SET clicks=webmaster_metrics_daily.clicks+1`,normalized)
	if err!=nil{return fmt.Errorf("record webmaster click: %w",err)}
	return nil
}

func (r *Repository) recordHostMetric(ctx context.Context,hosts []string,metric string) error {
	if r==nil || r.db==nil{return errors.New("webmaster repository is not initialized")}
	counts:=make(map[string]int64)
	for _,host:=range hosts{
		host=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host),"."))
		if host==""{continue}
		counts[host]++
	}
	if len(counts)==0{return nil}
	keys:=make([]string,0,len(counts));for host:=range counts{keys=append(keys,host)};sort.Strings(keys)
	values:=make([]int64,len(keys));for i,host:=range keys{values[i]=counts[host]}
	var q string
	switch metric{
	case "impressions":
		q=`WITH incoming(host,n) AS (SELECT * FROM unnest($1::text[],$2::bigint[])), matched AS (
SELECT s.site_id,sum(i.n)::bigint n FROM incoming i JOIN webmaster_sites s ON lower(s.host)=i.host WHERE s.status='VERIFIED' GROUP BY s.site_id)
INSERT INTO webmaster_metrics_daily(site_id,day,impressions)
SELECT site_id,current_date,n FROM matched
ON CONFLICT(site_id,day) DO UPDATE SET impressions=webmaster_metrics_daily.impressions+EXCLUDED.impressions`
	case "answer_citations":
		q=`WITH incoming(host,n) AS (SELECT * FROM unnest($1::text[],$2::bigint[])), matched AS (
SELECT s.site_id,sum(i.n)::bigint n FROM incoming i JOIN webmaster_sites s ON lower(s.host)=i.host WHERE s.status='VERIFIED' GROUP BY s.site_id)
INSERT INTO webmaster_metrics_daily(site_id,day,answer_citations)
SELECT site_id,current_date,n FROM matched
ON CONFLICT(site_id,day) DO UPDATE SET answer_citations=webmaster_metrics_daily.answer_citations+EXCLUDED.answer_citations`
	default:return fmt.Errorf("unknown webmaster metric %q",metric)
	}
	if _,err:=r.db.Exec(ctx,q,keys,values);err!=nil{return fmt.Errorf("record webmaster %s: %w",metric,err)}
	return nil
}
