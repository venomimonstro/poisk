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
)

type EntityStats struct{Organizations int `json:"organizations"`;Websites int `json:"websites"`;Published int `json:"published"`;Suppressed int `json:"suppressed"`;Changed int `json:"changed"`}

type entitySnapshot struct{
	Type PageType `json:"page_type"`
	PlaceID int64 `json:"place_id,omitempty"`
	DomainID int64 `json:"domain_id,omitempty"`
	Name string `json:"name,omitempty"`
	Host string `json:"host,omitempty"`
	CityKey string `json:"city_key,omitempty"`
	CategoryKey string `json:"category_key,omitempty"`
	Address string `json:"address,omitempty"`
	Website string `json:"website,omitempty"`
	LinkedOrganizations int `json:"linked_organizations,omitempty"`
	IndexedURLs int `json:"indexed_urls,omitempty"`
	Evidence Evidence `json:"evidence"`
	Gate Gate `json:"gate"`
	CanonicalPath string `json:"canonical_path"`
	Title string `json:"title"`
	MetaDescription string `json:"meta_description"`
	SourceUpdatedAt time.Time `json:"source_updated_at"`
}

func (r *Repository) RebuildEntities(ctx context.Context,limit int,now time.Time)(EntityStats,error){
	if r==nil||r.db==nil||limit<1||limit>10000{return EntityStats{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};now=now.UTC();stats:=EntityStats{}
	rows,err:=r.db.Query(ctx,`SELECT o.place_id,o.name,COALESCE(o.city_key,''),COALESCE(o.category_key,''),COALESCE(o.address,''),COALESCE(o.website,''),o.quality_score,o.source_count,o.updated_at
FROM organizations o WHERE o.status='ACTIVE' ORDER BY o.place_id LIMIT $1`,limit);if err!=nil{return stats,err}
	for rows.Next(){var placeID int64;var name,city,cat,address,website string;var quality float64;var sources int;var updated time.Time;if err:=rows.Scan(&placeID,&name,&city,&cat,&address,&website,&quality,&sources,&updated);err!=nil{rows.Close();return stats,err};e:=Evidence{Organizations:1,AverageQuality:quality,DistinctSources:sources};if website!=""{e.WithWebsite=1};if address!=""{e.WithAddress=1};gate:=Evaluate(PageOrganization,e);_,path,_:=Identity(PageOrganization,"","",placeID,0);snap:=entitySnapshot{Type:PageOrganization,PlaceID:placeID,Name:name,CityKey:city,CategoryKey:cat,Address:address,Website:website,Evidence:e,Gate:gate,CanonicalPath:path,Title:name,MetaDescription:organizationMeta(name,city,cat,address),SourceUpdatedAt:updated};state,changed,err:=r.applyEntitySnapshot(ctx,snap,now);if err!=nil{rows.Close();return stats,err};stats.Organizations++;if state=="PUBLISHED"{stats.Published++}else if state=="SUPPRESSED"{stats.Suppressed++};if changed{stats.Changed++}}
	if err:=rows.Err();err!=nil{rows.Close();return stats,err};rows.Close()

	rows,err=r.db.Query(ctx,`WITH linked AS (
 SELECT d.domain_id,d.host,d.updated_at,
        count(DISTINCT o.place_id)::int linked_organizations,
        count(DISTINCT o.place_id) FILTER(WHERE o.address IS NOT NULL AND btrim(o.address)<>'')::int with_address,
        COALESCE(avg(o.quality_score),0)::float8 average_quality,
        count(DISTINCT sl.source_key)::int distinct_sources
 FROM domains d
 JOIN organization_web_links wl ON wl.domain_id=d.domain_id AND wl.confidence>=80
 JOIN organizations o ON o.place_id=wl.place_id AND o.status='ACTIVE'
 LEFT JOIN organization_source_links sl ON sl.place_id=o.place_id
 WHERE d.status='ACTIVE' AND d.policy IN ('ALLOW','LIMITED')
 GROUP BY d.domain_id,d.host,d.updated_at
), indexed AS (
 SELECT domain_id,count(*) FILTER(WHERE index_status='INDEXED')::int indexed_urls FROM urls GROUP BY domain_id
)
SELECT l.domain_id,l.host,l.updated_at,l.linked_organizations,l.with_address,l.average_quality,l.distinct_sources,COALESCE(i.indexed_urls,0)
FROM linked l LEFT JOIN indexed i ON i.domain_id=l.domain_id ORDER BY l.domain_id LIMIT $1`,limit);if err!=nil{return stats,err}
	for rows.Next(){var domainID int64;var host string;var updated time.Time;var linked,withAddress,sources,indexed int;var quality float64;if err:=rows.Scan(&domainID,&host,&updated,&linked,&withAddress,&quality,&sources,&indexed);err!=nil{rows.Close();return stats,err};e:=Evidence{Organizations:linked,WithWebsite:linked,WithAddress:withAddress,AverageQuality:quality,DistinctSources:sources};gate:=Evaluate(PageWebsite,e);if indexed==0{gate.Publish=false;gate.Reason="no_indexed_urls"};_,path,_:=Identity(PageWebsite,"","",0,domainID);snap:=entitySnapshot{Type:PageWebsite,DomainID:domainID,Host:host,LinkedOrganizations:linked,IndexedURLs:indexed,Evidence:e,Gate:gate,CanonicalPath:path,Title:host,MetaDescription:websiteMeta(host,linked,indexed),SourceUpdatedAt:updated};state,changed,err:=r.applyEntitySnapshot(ctx,snap,now);if err!=nil{rows.Close();return stats,err};stats.Websites++;if state=="PUBLISHED"{stats.Published++}else if state=="SUPPRESSED"{stats.Suppressed++};if changed{stats.Changed++}}
	if err:=rows.Err();err!=nil{rows.Close();return stats,err};rows.Close();return stats,nil
}

func (r *Repository) applyEntitySnapshot(ctx context.Context,s entitySnapshot,now time.Time)(string,bool,error){
	var slug,path string;var err error
	if s.Type==PageOrganization{slug,path,err=Identity(s.Type,"","",s.PlaceID,0)}else{slug,path,err=Identity(s.Type,"","",0,s.DomainID)};if err!=nil{return "",false,err};s.CanonicalPath=path
	raw,err:=json.Marshal(s);if err!=nil{return "",false,err};sum:=sha256.Sum256(raw);hash:=hex.EncodeToString(sum[:])
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return "",false,err};defer func(){_=tx.Rollback(ctx)}()
	var pageID,version int64;var oldState string;var oldHash *string
	err=tx.QueryRow(ctx,`SELECT page_id,version,state,content_hash FROM datahub_pages WHERE page_type=$1 AND slug=$2 FOR UPDATE`,string(s.Type),slug).Scan(&pageID,&version,&oldState,&oldHash)
	state:="DRAFT";if s.Gate.Publish{state="PUBLISHED"}
	if errors.Is(err,pgx.ErrNoRows){var published any=nil;if state=="PUBLISHED"{published=now};err=tx.QueryRow(ctx,`INSERT INTO datahub_pages(page_type,place_id,domain_id,slug,canonical_path,state,version,evidence_count,quality_score,content_hash,title,meta_description,published_at,refreshed_at) VALUES($1,NULLIF($2,0),NULLIF($3,0),$4,$5,$6,1,$7,$8,$9,$10,$11,$12,$13) RETURNING page_id,version`,string(s.Type),s.PlaceID,s.DomainID,slug,path,state,s.Evidence.Organizations,s.Gate.Score,hash,s.Title,s.MetaDescription,published,now).Scan(&pageID,&version);if err!=nil{return "",false,err};if _,err=tx.Exec(ctx,`INSERT INTO datahub_page_versions(page_id,version,content_hash,evidence_count,quality_score,snapshot) VALUES($1,$2,$3,$4,$5,$6::jsonb)`,pageID,version,hash,s.Evidence.Organizations,s.Gate.Score,string(raw));err!=nil{return "",false,err};action:="BUILD";if state=="PUBLISHED"{action="PUBLISH"};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,to_version,reason) VALUES($1,$2,$3,$4)`,pageID,action,version,s.Gate.Reason);err!=nil{return "",false,err};if err=tx.Commit(ctx);err!=nil{return "",false,err};return state,true,nil}
	if err!=nil{return "",false,err};if !s.Gate.Publish&&(oldState=="PUBLISHED"||oldState=="SUPPRESSED"){state="SUPPRESSED"}
	if oldHash!=nil&&*oldHash==hash&&oldState==state{_,err=tx.Exec(ctx,`UPDATE datahub_pages SET refreshed_at=$2,updated_at=now() WHERE page_id=$1`,pageID,now);if err!=nil{return "",false,err};if err=tx.Commit(ctx);err!=nil{return "",false,err};return state,false,nil}
	newVersion:=version+1;_,err=tx.Exec(ctx,`UPDATE datahub_pages SET state=$2,version=$3,evidence_count=$4,quality_score=$5,content_hash=$6,title=$7,meta_description=$8,published_at=CASE WHEN $2='PUBLISHED' THEN COALESCE(published_at,$9) ELSE published_at END,refreshed_at=$9,updated_at=now() WHERE page_id=$1`,pageID,state,newVersion,s.Evidence.Organizations,s.Gate.Score,hash,s.Title,s.MetaDescription,now);if err!=nil{return "",false,err};if _,err=tx.Exec(ctx,`INSERT INTO datahub_page_versions(page_id,version,content_hash,evidence_count,quality_score,snapshot) VALUES($1,$2,$3,$4,$5,$6::jsonb)`,pageID,newVersion,hash,s.Evidence.Organizations,s.Gate.Score,string(raw));err!=nil{return "",false,err};action:="BUILD";if state=="PUBLISHED"&&oldState!="PUBLISHED"{action="PUBLISH"}else if oldState=="PUBLISHED"&&state!="PUBLISHED"{action="UNPUBLISH"};if _,err=tx.Exec(ctx,`INSERT INTO datahub_publication_events(page_id,action,from_version,to_version,reason) VALUES($1,$2,$3,$4,$5)`,pageID,action,version,newVersion,s.Gate.Reason);err!=nil{return "",false,err};if err=tx.Commit(ctx);err!=nil{return "",false,err};return state,true,nil
}

func organizationMeta(name,city,cat,address string)string{parts:=[]string{name};if cat!=""{parts=append(parts,displayKey(cat))};if city!=""{parts=append(parts,displayKey(city))};if address!=""{parts=append(parts,address)};s:="Карточка организации: "+strings.Join(parts," — ")+". Данные из канонического каталога Poisk.";return trimRunes(s,320)}
func websiteMeta(host string,linked,indexed int)string{return trimRunes(fmt.Sprintf("Сайт %s в каталоге Poisk: %d связанных организаций и %d проиндексированных страниц.",host,linked,indexed),320)}
func trimRunes(s string,n int)string{r:=[]rune(strings.TrimSpace(s));if len(r)<=n{return string(r)};return string(r[:n])}
