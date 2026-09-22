package datahub

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Page struct{
	ID int64 `json:"page_id"`
	Type PageType `json:"page_type"`
	Slug string `json:"slug"`
	CanonicalPath string `json:"canonical_path"`
	Title string `json:"title"`
	MetaDescription string `json:"meta_description"`
	Version int64 `json:"version"`
	EvidenceCount int `json:"evidence_count"`
	QualityScore int `json:"quality_score"`
	RefreshedAt *time.Time `json:"refreshed_at,omitempty"`
	Snapshot json.RawMessage `json:"snapshot"`
	Related []PageLink `json:"related"`
}

type PageLink struct{Path string `json:"path"`;Title string `json:"title"`;Type PageType `json:"page_type"`}
type OrganizationItem struct{PlaceID int64 `json:"place_id"`;Name string `json:"name"`;CityKey string `json:"city_key,omitempty"`;CategoryKey string `json:"category_key,omitempty"`;Address string `json:"address,omitempty"`;Website string `json:"website,omitempty"`;QualityScore float64 `json:"quality_score"`}
type WebsiteItem struct{DomainID int64 `json:"domain_id"`;Host string `json:"host"`;LinkedOrganizations int `json:"linked_organizations"`;IndexedURLs int `json:"indexed_urls"`}
type SitemapItem struct{PageID int64 `json:"page_id"`;Path string `json:"path"`;UpdatedAt time.Time `json:"updated_at"`}

func (r *Repository) PublishedPage(ctx context.Context,path string)(Page,error){
	path=strings.TrimSpace(path);if r==nil||r.db==nil||path==""||len(path)>256{return Page{},ErrInvalid};var p Page;var raw []byte
	err:=r.db.QueryRow(ctx,`SELECT p.page_id,p.page_type,p.slug,p.canonical_path,p.title,p.meta_description,p.version,p.evidence_count,p.quality_score,p.refreshed_at,v.snapshot
FROM datahub_pages p JOIN datahub_page_versions v ON v.page_id=p.page_id AND v.version=p.version
WHERE p.canonical_path=$1 AND p.state='PUBLISHED'`,path).Scan(&p.ID,&p.Type,&p.Slug,&p.CanonicalPath,&p.Title,&p.MetaDescription,&p.Version,&p.EvidenceCount,&p.QualityScore,&p.RefreshedAt,&raw);if errors.Is(err,pgx.ErrNoRows){return Page{},pgx.ErrNoRows};if err!=nil{return Page{},err};p.Snapshot=append(json.RawMessage(nil),raw...)
	p.Related,err=r.related(ctx,p.ID,p.Type,path,8);if err!=nil{return Page{},err};return p,nil
}

func (r *Repository) related(ctx context.Context,pageID int64,t PageType,path string,limit int)([]PageLink,error){
	if limit<1||limit>12{return nil,ErrInvalid}
	rows,err:=r.db.Query(ctx,`WITH current AS (SELECT city_key,category_key FROM datahub_pages WHERE page_id=$1)
SELECT p.canonical_path,p.title,p.page_type
FROM datahub_pages p,current c
WHERE p.state='PUBLISHED' AND p.page_id<>$1
  AND (
    (c.city_key IS NOT NULL AND p.city_key=c.city_key) OR
    (c.category_key IS NOT NULL AND p.category_key=c.category_key) OR
    (p.page_type IN ('ORGANIZATION','WEBSITE') AND p.updated_at>=now()-interval '30 days')
  )
ORDER BY CASE p.page_type WHEN $2 THEN 0 WHEN 'CITY_CATEGORY' THEN 1 WHEN 'ORGANIZATION' THEN 2 ELSE 3 END,p.quality_score DESC,p.page_id
LIMIT $3`,pageID,string(t),limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]PageLink,0,limit);for rows.Next(){var x PageLink;if err:=rows.Scan(&x.Path,&x.Title,&x.Type);err!=nil{return nil,err};if x.Path!=path{out=append(out,x)}};return out,rows.Err()
}

func (r *Repository) Organizations(ctx context.Context,cityKey,categoryKey string,after int64,limit int)([]OrganizationItem,int64,error){
	cityKey=strings.TrimSpace(cityKey);categoryKey=strings.TrimSpace(categoryKey);if r==nil||r.db==nil||after<0||limit<1||limit>50{return nil,0,ErrInvalid}
	rows,err:=r.db.Query(ctx,`SELECT place_id,name,COALESCE(city_key,''),COALESCE(category_key,''),COALESCE(address,''),COALESCE(website,''),quality_score
FROM organizations
WHERE status='ACTIVE' AND place_id>$1
  AND ($2='' OR city_key=$2)
  AND ($3='' OR category_key=$3)
ORDER BY place_id LIMIT $4`,after,cityKey,categoryKey,limit+1);if err!=nil{return nil,0,err};defer rows.Close();all:=make([]OrganizationItem,0,limit+1);for rows.Next(){var x OrganizationItem;if err:=rows.Scan(&x.PlaceID,&x.Name,&x.CityKey,&x.CategoryKey,&x.Address,&x.Website,&x.QualityScore);err!=nil{return nil,0,err};all=append(all,x)};if err:=rows.Err();err!=nil{return nil,0,err};next:=int64(0);if len(all)>limit{next=all[limit-1].PlaceID;all=all[:limit]};return all,next,nil
}

func (r *Repository) Websites(ctx context.Context,after int64,limit int)([]WebsiteItem,int64,error){
	if r==nil||r.db==nil||after<0||limit<1||limit>50{return nil,0,ErrInvalid}
	rows,err:=r.db.Query(ctx,`SELECT p.domain_id,d.host,
 COALESCE((v.snapshot->>'linked_organizations')::int,0),COALESCE((v.snapshot->>'indexed_urls')::int,0)
FROM datahub_pages p JOIN domains d ON d.domain_id=p.domain_id
JOIN datahub_page_versions v ON v.page_id=p.page_id AND v.version=p.version
WHERE p.page_type='WEBSITE' AND p.state='PUBLISHED' AND p.domain_id>$1
ORDER BY p.domain_id LIMIT $2`,after,limit+1);if err!=nil{return nil,0,err};defer rows.Close();all:=make([]WebsiteItem,0,limit+1);for rows.Next(){var x WebsiteItem;if err:=rows.Scan(&x.DomainID,&x.Host,&x.LinkedOrganizations,&x.IndexedURLs);err!=nil{return nil,0,err};all=append(all,x)};if err:=rows.Err();err!=nil{return nil,0,err};next:=int64(0);if len(all)>limit{next=all[limit-1].DomainID;all=all[:limit]};return all,next,nil
}

func (r *Repository) Sitemap(ctx context.Context,after int64,limit int)([]SitemapItem,int64,error){
	if r==nil||r.db==nil||after<0||limit<1||limit>500{return nil,0,ErrInvalid};rows,err:=r.db.Query(ctx,`SELECT page_id,canonical_path,updated_at FROM datahub_pages WHERE state='PUBLISHED' AND page_id>$1 ORDER BY page_id LIMIT $2`,after,limit+1);if err!=nil{return nil,0,err};defer rows.Close();all:=make([]SitemapItem,0,limit+1);for rows.Next(){var x SitemapItem;if err:=rows.Scan(&x.PageID,&x.Path,&x.UpdatedAt);err!=nil{return nil,0,err};all=append(all,x)};if err:=rows.Err();err!=nil{return nil,0,err};next:=int64(0);if len(all)>limit{next=all[limit-1].PageID;all=all[:limit]};return all,next,nil
}
