package datahub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Aggregate struct{
	Type PageType `json:"page_type"`
	CityKey string `json:"city_key,omitempty"`
	CategoryKey string `json:"category_key,omitempty"`
	Organizations int `json:"organizations"`
	WithWebsite int `json:"with_website"`
	WithAddress int `json:"with_address"`
	AverageQuality float64 `json:"average_quality"`
	DistinctSources int `json:"distinct_sources"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BuildStats struct{Scanned int `json:"scanned"`;Published int `json:"published"`;Draft int `json:"draft"`;Suppressed int `json:"suppressed"`;Changed int `json:"changed"`}

func (r *Repository) RebuildDirectories(ctx context.Context,limit int,now time.Time)(BuildStats,error){
	if r==nil||r.db==nil||limit<1||limit>10000{return BuildStats{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};now=now.UTC()
	items,err:=r.directoryAggregates(ctx,limit,now);if err!=nil{return BuildStats{},err};stats:=BuildStats{Scanned:len(items)}
	for _,a:=range items{state,changed,err:=r.applyAggregate(ctx,a,now);if err!=nil{return stats,err};if changed{stats.Changed++};switch state{case "PUBLISHED":stats.Published++;case "SUPPRESSED":stats.Suppressed++;default:stats.Draft++}}
	return stats,nil
}

func (r *Repository) directoryAggregates(ctx context.Context,limit int,now time.Time)([]Aggregate,error){
	const q=`WITH active_orgs AS (
 SELECT o.place_id,o.city_key,o.category_key,o.website,o.address,o.quality_score,o.updated_at
 FROM organizations o WHERE o.status='ACTIVE'
), city_stats AS (
 SELECT city_key,count(*)::int organizations,
        count(*) FILTER(WHERE website IS NOT NULL AND btrim(website)<>'')::int with_website,
        count(*) FILTER(WHERE address IS NOT NULL AND btrim(address)<>'')::int with_address,
        COALESCE(avg(quality_score),0)::float8 average_quality,max(updated_at) updated_at
 FROM active_orgs WHERE city_key IS NOT NULL GROUP BY city_key
), city_sources AS (
 SELECT o.city_key,count(DISTINCT l.source_key)::int distinct_sources
 FROM active_orgs o JOIN organization_source_links l ON l.place_id=o.place_id
 WHERE o.city_key IS NOT NULL GROUP BY o.city_key
), category_stats AS (
 SELECT category_key,count(*)::int organizations,
        count(*) FILTER(WHERE website IS NOT NULL AND btrim(website)<>'')::int with_website,
        count(*) FILTER(WHERE address IS NOT NULL AND btrim(address)<>'')::int with_address,
        COALESCE(avg(quality_score),0)::float8 average_quality,max(updated_at) updated_at
 FROM active_orgs WHERE category_key IS NOT NULL GROUP BY category_key
), category_sources AS (
 SELECT o.category_key,count(DISTINCT l.source_key)::int distinct_sources
 FROM active_orgs o JOIN organization_source_links l ON l.place_id=o.place_id
 WHERE o.category_key IS NOT NULL GROUP BY o.category_key
), cc_stats AS (
 SELECT city_key,category_key,count(*)::int organizations,
        count(*) FILTER(WHERE website IS NOT NULL AND btrim(website)<>'')::int with_website,
        count(*) FILTER(WHERE address IS NOT NULL AND btrim(address)<>'')::int with_address,
        COALESCE(avg(quality_score),0)::float8 average_quality,max(updated_at) updated_at
 FROM active_orgs WHERE city_key IS NOT NULL AND category_key IS NOT NULL GROUP BY city_key,category_key
), cc_sources AS (
 SELECT o.city_key,o.category_key,count(DISTINCT l.source_key)::int distinct_sources
 FROM active_orgs o JOIN organization_source_links l ON l.place_id=o.place_id
 WHERE o.city_key IS NOT NULL AND o.category_key IS NOT NULL GROUP BY o.city_key,o.category_key
), aggregates AS (
 SELECT 'CITY'::text page_type,c.city_key,NULL::text category_key,c.organizations,c.with_website,c.with_address,c.average_quality,COALESCE(s.distinct_sources,0) distinct_sources,c.updated_at
 FROM city_stats c LEFT JOIN city_sources s USING(city_key)
 UNION ALL
 SELECT 'CATEGORY',NULL,c.category_key,c.organizations,c.with_website,c.with_address,c.average_quality,COALESCE(s.distinct_sources,0),c.updated_at
 FROM category_stats c LEFT JOIN category_sources s USING(category_key)
 UNION ALL
 SELECT 'CITY_CATEGORY',c.city_key,c.category_key,c.organizations,c.with_website,c.with_address,c.average_quality,COALESCE(s.distinct_sources,0),c.updated_at
 FROM cc_stats c LEFT JOIN cc_sources s USING(city_key,category_key)
)
SELECT page_type,COALESCE(city_key,''),COALESCE(category_key,''),organizations,with_website,with_address,average_quality,distinct_sources,COALESCE(updated_at,$2)
FROM aggregates ORDER BY page_type,city_key NULLS FIRST,category_key NULLS FIRST LIMIT $1`
	rows,err:=r.db.Query(ctx,q,limit,now);if err!=nil{return nil,err};defer rows.Close();out:=make([]Aggregate,0,limit)
	for rows.Next(){var a Aggregate;var t string;if err:=rows.Scan(&t,&a.CityKey,&a.CategoryKey,&a.Organizations,&a.WithWebsite,&a.WithAddress,&a.AverageQuality,&a.DistinctSources,&a.UpdatedAt);err!=nil{return nil,err};a.Type=PageType(t);out=append(out,a)};return out,rows.Err()
}

func (r *Repository) applyAggregate(ctx context.Context,a Aggregate,now time.Time)(string,bool,error){
	slug,path,err:=Identity(a.Type,a.CityKey,a.CategoryKey,0,0);if err!=nil{return "",false,err}
	gate:=Evaluate(a.Type,Evidence{Organizations:a.Organizations,WithWebsite:a.WithWebsite,WithAddress:a.WithAddress,AverageQuality:a.AverageQuality,DistinctSources:a.DistinctSources})
	title,meta:=directoryMetadata(a)
	snapshot:=struct{Aggregate Aggregate `json:"aggregate"`;Gate Gate `json:"gate"`;CanonicalPath string `json:"canonical_path"`;Title string `json:"title"`;MetaDescription string `json:"meta_description"`}{a,gate,path,title,meta}
	raw,err:=json.Marshal(snapshot);if err!=nil{return "",false,err};sum:=sha256.Sum256(raw);hash:=hex.EncodeToString(sum[:])
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return "",false,err};defer func(){_=tx.Rollback(ctx)}()
	var pageID,version int64;var oldState string;var oldHash *string
	err=tx.QueryRow(ctx,`SELECT page_id,version,state,content_hash FROM datahub_pages WHERE page_type=$1 AND slug=$2 FOR UPDATE`,string(a.Type),slug).Scan(&pageID,&version,&oldState,&oldHash)
	if errors.Is(err,pgx.ErrNoRows){
		state:="DRAFT";var published any=nil;if gate.Publish{state="PUBLISHED";published=now}
		err=tx.QueryRow(ctx,`INSERT INTO datahub_pages(page_type,city_key,category_key,slug,canonical_path,state,version,evidence_count,quality_score,content_hash,title,meta_description,published_at,refreshed_at)
VALUES($1,NULLIF($2,''),NULLIF($3,''),$4,$5,$6,1,$7,$8,$9,$10,$11,$12,$13) RETURNING page_id,version`,string(a.Type),a.CityKey,a.CategoryKey,slug,path,state,a.Organizations,gate.Score,hash,title,meta,published,now).Scan(&pageID,&version);if err!=nil{return "",false,err}
		if _,err=tx.Exec(ctx,`INSERT INTO datahub_page_versions(page_id,version,content_hash,evidence_count,quality_score,snapshot) VALUES($1,$2,$3,$4,$5,$6::jsonb)`,pageID,version,hash,a.Organizations,gate.Score,string(raw));err!=nil{return "",false,err}
		action:="BUILD";if state=="PUBLISHED"{action="PUBLISH"};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,to_version,reason) VALUES($1,$2,$3,$4)`,pageID,action,version,gate.Reason);err!=nil{return "",false,err}
		if err=tx.Commit(ctx);err!=nil{return "",false,err};return state,true,nil
	}
	if err!=nil{return "",false,err}
	newState:="DRAFT";if gate.Publish{newState="PUBLISHED"}else if oldState=="PUBLISHED"||oldState=="SUPPRESSED"{newState="SUPPRESSED"}
	if oldHash!=nil&&*oldHash==hash&&oldState==newState{_,err=tx.Exec(ctx,`UPDATE datahub_pages SET refreshed_at=$2,updated_at=now() WHERE page_id=$1`,pageID,now);if err!=nil{return "",false,err};if err=tx.Commit(ctx);err!=nil{return "",false,err};return newState,false,nil}
	newVersion:=version+1
	_,err=tx.Exec(ctx,`UPDATE datahub_pages SET state=$2,version=$3,evidence_count=$4,quality_score=$5,content_hash=$6,title=$7,meta_description=$8,published_at=CASE WHEN $2='PUBLISHED' THEN COALESCE(published_at,$9) ELSE published_at END,refreshed_at=$9,updated_at=now() WHERE page_id=$1`,pageID,newState,newVersion,a.Organizations,gate.Score,hash,title,meta,now);if err!=nil{return "",false,err}
	if _,err=tx.Exec(ctx,`INSERT INTO datahub_page_versions(page_id,version,content_hash,evidence_count,quality_score,snapshot) VALUES($1,$2,$3,$4,$5,$6::jsonb)`,pageID,newVersion,hash,a.Organizations,gate.Score,string(raw));err!=nil{return "",false,err}
	action:="BUILD";if newState=="PUBLISHED"&&oldState!="PUBLISHED"{action="PUBLISH"}else if oldState=="PUBLISHED"&&newState!="PUBLISHED"{action="UNPUBLISH"}
	if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason) VALUES($1,$2,$3,$4,$5)`,pageID,action,version,newVersion,gate.Reason);err!=nil{return "",false,err}
	if err=tx.Commit(ctx);err!=nil{return "",false,err};return newState,true,nil
}

func directoryMetadata(a Aggregate)(string,string){
	city:=displayKey(a.CityKey);cat:=displayKey(a.CategoryKey)
	switch a.Type{
	case PageCity:return fmt.Sprintf("Организации — %s",city),fmt.Sprintf("Каталог организаций в %s: %d активных организаций по данным Poisk.",city,a.Organizations)
	case PageCategory:return fmt.Sprintf("Категория: %s",cat),fmt.Sprintf("Каталог организаций категории %s: %d активных организаций по данным Poisk.",cat,a.Organizations)
	default:return fmt.Sprintf("%s — %s",cat,city),fmt.Sprintf("Каталог организаций категории %s в %s: %d активных организаций по данным Poisk.",cat,city,a.Organizations)
	}
}

func displayKey(v string)string{v=strings.TrimSpace(v);if v==""{return "каталог"};r:=strings.NewReplacer("-"," ","_"," ","."," ");return strings.Join(strings.Fields(r.Replace(v))," ")}
