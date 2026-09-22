package datahub

import (
	"context"
	"strconv"
	"time"
)

type BatchStats struct{
	Job string `json:"job"`
	Scanned int `json:"scanned"`
	Changed int `json:"changed"`
	Published int `json:"published"`
	Suppressed int `json:"suppressed"`
	Completed bool `json:"completed"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func (r *Repository) RebuildDirectoryBatch(ctx context.Context,limit int,now time.Time)(BatchStats,error){
	if r==nil||r.db==nil||limit<1||limit>1000{return BatchStats{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};cursor,err:=r.buildCursor(ctx,"DIRECTORIES");if err!=nil{return BatchStats{},err}
	const q=`WITH active_orgs AS (
 SELECT o.place_id,o.city_key,o.category_key,o.website,o.address,o.quality_score,o.updated_at
 FROM organizations o WHERE o.status='ACTIVE'
), city_stats AS (
 SELECT city_key,count(*)::int organizations,count(*) FILTER(WHERE website IS NOT NULL AND btrim(website)<>'')::int with_website,count(*) FILTER(WHERE address IS NOT NULL AND btrim(address)<>'')::int with_address,COALESCE(avg(quality_score),0)::float8 average_quality,max(updated_at) updated_at
 FROM active_orgs WHERE city_key IS NOT NULL GROUP BY city_key
), city_sources AS (
 SELECT o.city_key,count(DISTINCT l.source_key)::int distinct_sources FROM active_orgs o JOIN organization_source_links l ON l.place_id=o.place_id WHERE o.city_key IS NOT NULL GROUP BY o.city_key
), category_stats AS (
 SELECT category_key,count(*)::int organizations,count(*) FILTER(WHERE website IS NOT NULL AND btrim(website)<>'')::int with_website,count(*) FILTER(WHERE address IS NOT NULL AND btrim(address)<>'')::int with_address,COALESCE(avg(quality_score),0)::float8 average_quality,max(updated_at) updated_at
 FROM active_orgs WHERE category_key IS NOT NULL GROUP BY category_key
), category_sources AS (
 SELECT o.category_key,count(DISTINCT l.source_key)::int distinct_sources FROM active_orgs o JOIN organization_source_links l ON l.place_id=o.place_id WHERE o.category_key IS NOT NULL GROUP BY o.category_key
), cc_stats AS (
 SELECT city_key,category_key,count(*)::int organizations,count(*) FILTER(WHERE website IS NOT NULL AND btrim(website)<>'')::int with_website,count(*) FILTER(WHERE address IS NOT NULL AND btrim(address)<>'')::int with_address,COALESCE(avg(quality_score),0)::float8 average_quality,max(updated_at) updated_at
 FROM active_orgs WHERE city_key IS NOT NULL AND category_key IS NOT NULL GROUP BY city_key,category_key
), cc_sources AS (
 SELECT o.city_key,o.category_key,count(DISTINCT l.source_key)::int distinct_sources FROM active_orgs o JOIN organization_source_links l ON l.place_id=o.place_id WHERE o.city_key IS NOT NULL AND o.category_key IS NOT NULL GROUP BY o.city_key,o.category_key
), aggregates AS (
 SELECT 'CITY'::text page_type,c.city_key,NULL::text category_key,c.organizations,c.with_website,c.with_address,c.average_quality,COALESCE(s.distinct_sources,0) distinct_sources,c.updated_at FROM city_stats c LEFT JOIN city_sources s USING(city_key)
 UNION ALL SELECT 'CATEGORY',NULL,c.category_key,c.organizations,c.with_website,c.with_address,c.average_quality,COALESCE(s.distinct_sources,0),c.updated_at FROM category_stats c LEFT JOIN category_sources s USING(category_key)
 UNION ALL SELECT 'CITY_CATEGORY',c.city_key,c.category_key,c.organizations,c.with_website,c.with_address,c.average_quality,COALESCE(s.distinct_sources,0),c.updated_at FROM cc_stats c LEFT JOIN cc_sources s USING(city_key,category_key)
), keyed AS (
 SELECT *,concat(page_type,'|',COALESCE(city_key,''),'|',COALESCE(category_key,'')) cursor_key FROM aggregates
)
SELECT page_type,COALESCE(city_key,''),COALESCE(category_key,''),organizations,with_website,with_address,average_quality,distinct_sources,COALESCE(updated_at,$3),cursor_key
FROM keyed WHERE cursor_key>$1 ORDER BY cursor_key LIMIT $2`
	rows,err:=r.db.Query(ctx,q,cursor,limit,now);if err!=nil{return BatchStats{},err};items:=make([]struct{a Aggregate;cursor string},0,limit)
	for rows.Next(){var x struct{a Aggregate;cursor string};var typ string;if err:=rows.Scan(&typ,&x.a.CityKey,&x.a.CategoryKey,&x.a.Organizations,&x.a.WithWebsite,&x.a.WithAddress,&x.a.AverageQuality,&x.a.DistinctSources,&x.a.UpdatedAt,&x.cursor);err!=nil{rows.Close();return BatchStats{},err};x.a.Type=PageType(typ);items=append(items,x)};if err:=rows.Err();err!=nil{rows.Close();return BatchStats{},err};rows.Close()
	stats:=BatchStats{Job:"DIRECTORIES",Scanned:len(items)};if len(items)==0{stats.Completed=true;if err:=r.saveBuildCursor(ctx,"DIRECTORIES","",now,true);err!=nil{return stats,err};return stats,nil}
	for _,x:=range items{state,changed,err:=r.applyAggregate(ctx,x.a,now);if err!=nil{return stats,err};stats.NextCursor=x.cursor;if changed{stats.Changed++};if state=="PUBLISHED"{stats.Published++}else if state=="SUPPRESSED"{stats.Suppressed++}}
	if err:=r.saveBuildCursor(ctx,"DIRECTORIES",stats.NextCursor,now,false);err!=nil{return stats,err};return stats,nil
}

func (r *Repository) RebuildOrganizationBatch(ctx context.Context,limit int,now time.Time)(BatchStats,error){
	if r==nil||r.db==nil||limit<1||limit>1000{return BatchStats{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};cursor,err:=r.buildCursor(ctx,"ORGANIZATIONS");if err!=nil{return BatchStats{},err};after,_:=strconv.ParseInt(cursor,10,64)
	rows,err:=r.db.Query(ctx,`SELECT o.place_id,o.name,COALESCE(o.city_key,''),COALESCE(o.category_key,''),COALESCE(o.address,''),COALESCE(o.website,''),o.quality_score,o.source_count,o.updated_at FROM organizations o WHERE o.status='ACTIVE' AND o.place_id>$1 ORDER BY o.place_id LIMIT $2`,after,limit);if err!=nil{return BatchStats{},err}
	type item struct{place int64;name,city,cat,address,website string;quality float64;sources int;updated time.Time};items:=make([]item,0,limit);for rows.Next(){var x item;if err:=rows.Scan(&x.place,&x.name,&x.city,&x.cat,&x.address,&x.website,&x.quality,&x.sources,&x.updated);err!=nil{rows.Close();return BatchStats{},err};items=append(items,x)};if err:=rows.Err();err!=nil{rows.Close();return BatchStats{},err};rows.Close()
	stats:=BatchStats{Job:"ORGANIZATIONS",Scanned:len(items)};if len(items)==0{stats.Completed=true;if err:=r.saveBuildCursor(ctx,"ORGANIZATIONS","",now,true);err!=nil{return stats,err};return stats,nil}
	for _,x:=range items{e:=Evidence{Organizations:1,AverageQuality:x.quality,DistinctSources:x.sources};if x.website!=""{e.WithWebsite=1};if x.address!=""{e.WithAddress=1};gate:=Evaluate(PageOrganization,e);_,path,_:=Identity(PageOrganization,"","",x.place,0);snap:=entitySnapshot{Type:PageOrganization,PlaceID:x.place,Name:x.name,CityKey:x.city,CategoryKey:x.cat,Address:x.address,Website:x.website,Evidence:e,Gate:gate,CanonicalPath:path,Title:"Организация: "+x.name,MetaDescription:organizationMeta(x.name,x.city,x.cat,x.address),SourceUpdatedAt:x.updated};state,changed,err:=r.applyEntitySnapshot(ctx,snap,now);if err!=nil{return stats,err};stats.NextCursor=strconv.FormatInt(x.place,10);if changed{stats.Changed++};if state=="PUBLISHED"{stats.Published++}else if state=="SUPPRESSED"{stats.Suppressed++}}
	if err:=r.saveBuildCursor(ctx,"ORGANIZATIONS",stats.NextCursor,now,false);err!=nil{return stats,err};return stats,nil
}

func (r *Repository) RebuildWebsiteBatch(ctx context.Context,limit int,now time.Time)(BatchStats,error){
	if r==nil||r.db==nil||limit<1||limit>1000{return BatchStats{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};cursor,err:=r.buildCursor(ctx,"WEBSITES");if err!=nil{return BatchStats{},err};after,_:=strconv.ParseInt(cursor,10,64)
	rows,err:=r.db.Query(ctx,`WITH linked AS (
 SELECT d.domain_id,d.host,d.updated_at,count(DISTINCT o.place_id)::int linked_organizations,count(DISTINCT o.place_id) FILTER(WHERE o.address IS NOT NULL AND btrim(o.address)<>'')::int with_address,COALESCE(avg(o.quality_score),0)::float8 average_quality,count(DISTINCT sl.source_key)::int distinct_sources
 FROM domains d JOIN organization_web_links wl ON wl.domain_id=d.domain_id AND wl.confidence>=80 JOIN organizations o ON o.place_id=wl.place_id AND o.status='ACTIVE' LEFT JOIN organization_source_links sl ON sl.place_id=o.place_id
 WHERE d.status='ACTIVE' AND d.policy IN ('ALLOW','LIMITED') AND d.domain_id>$1 GROUP BY d.domain_id,d.host,d.updated_at
), indexed AS (SELECT domain_id,count(*) FILTER(WHERE index_status='INDEXED')::int indexed_urls FROM urls GROUP BY domain_id)
SELECT l.domain_id,l.host,l.updated_at,l.linked_organizations,l.with_address,l.average_quality,l.distinct_sources,COALESCE(i.indexed_urls,0) FROM linked l LEFT JOIN indexed i ON i.domain_id=l.domain_id ORDER BY l.domain_id LIMIT $2`,after,limit);if err!=nil{return BatchStats{},err}
	type item struct{domain int64;host string;updated time.Time;linked,addresses,sources,indexed int;quality float64};items:=make([]item,0,limit);for rows.Next(){var x item;if err:=rows.Scan(&x.domain,&x.host,&x.updated,&x.linked,&x.addresses,&x.quality,&x.sources,&x.indexed);err!=nil{rows.Close();return BatchStats{},err};items=append(items,x)};if err:=rows.Err();err!=nil{rows.Close();return BatchStats{},err};rows.Close()
	stats:=BatchStats{Job:"WEBSITES",Scanned:len(items)};if len(items)==0{stats.Completed=true;if err:=r.saveBuildCursor(ctx,"WEBSITES","",now,true);err!=nil{return stats,err};return stats,nil}
	for _,x:=range items{e:=Evidence{Organizations:x.linked,WithWebsite:x.linked,WithAddress:x.addresses,AverageQuality:x.quality,DistinctSources:x.sources};gate:=Evaluate(PageWebsite,e);if x.indexed==0{gate.Publish=false;gate.Reason="no_indexed_urls"};_,path,_:=Identity(PageWebsite,"","",0,x.domain);snap:=entitySnapshot{Type:PageWebsite,DomainID:x.domain,Host:x.host,LinkedOrganizations:x.linked,IndexedURLs:x.indexed,Evidence:e,Gate:gate,CanonicalPath:path,Title:"Сайт "+x.host,MetaDescription:websiteMeta(x.host,x.linked,x.indexed),SourceUpdatedAt:x.updated};state,changed,err:=r.applyEntitySnapshot(ctx,snap,now);if err!=nil{return stats,err};stats.NextCursor=strconv.FormatInt(x.domain,10);if changed{stats.Changed++};if state=="PUBLISHED"{stats.Published++}else if state=="SUPPRESSED"{stats.Suppressed++}}
	if err:=r.saveBuildCursor(ctx,"WEBSITES",stats.NextCursor,now,false);err!=nil{return stats,err};return stats,nil
}

func (r *Repository) buildCursor(ctx context.Context,key string)(string,error){var cursor string;err:=r.db.QueryRow(ctx,`SELECT cursor_text FROM datahub_build_state WHERE job_key=$1`,key).Scan(&cursor);return cursor,err}
func (r *Repository) saveBuildCursor(ctx context.Context,key,cursor string,now time.Time,completed bool)error{_,err:=r.db.Exec(ctx,`UPDATE datahub_build_state SET cursor_text=$2,last_run_at=$3,last_completed_at=CASE WHEN $4 THEN $3 ELSE last_completed_at END,updated_at=now() WHERE job_key=$1`,key,cursor,now,completed);return err}
