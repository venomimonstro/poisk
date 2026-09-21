package widget

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("site search widget not found")
	ErrForbidden = errors.New("site search widget forbidden")
	ErrInvalid = errors.New("invalid site search widget input")
)

type Repository struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Config struct {
	WidgetID int64 `json:"widget_id"`
	SiteID int64 `json:"site_id"`
	OwnerUserID int64 `json:"-"`
	PublicKey string `json:"public_key"`
	Origin string `json:"origin"`
	Host string `json:"host"`
	Status string `json:"status"`
	MaxResults int `json:"max_results"`
	CreatedAt time.Time `json:"created_at"`
}

func newPublicKey()(string,error){
	raw:=make([]byte,24);if _,err:=rand.Read(raw);err!=nil{return "",err}
	return "psw_"+base64.RawURLEncoding.EncodeToString(raw),nil
}

func (r *Repository) Ensure(ctx context.Context,userID,siteID int64)(Config,error){
	if r==nil||r.db==nil||userID<=0||siteID<=0{return Config{},ErrInvalid}
	key,err:=newPublicKey();if err!=nil{return Config{},err}
	var out Config
	err=r.db.QueryRow(ctx,`WITH owned AS (
 SELECT s.site_id,s.user_id,s.origin,s.host FROM webmaster_sites s JOIN domains d ON d.domain_id=s.domain_id
 WHERE s.site_id=$2 AND s.user_id=$1 AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK'
), upserted AS (
 INSERT INTO site_search_widgets(site_id,public_key,status)
 SELECT site_id,$3,'ACTIVE' FROM owned
 ON CONFLICT(site_id) DO UPDATE SET
   public_key=CASE WHEN site_search_widgets.status='REVOKED' THEN EXCLUDED.public_key ELSE site_search_widgets.public_key END,
   rotated_at=CASE WHEN site_search_widgets.status='REVOKED' THEN now() ELSE site_search_widgets.rotated_at END,
   status='ACTIVE',revoked_at=NULL,updated_at=now()
 RETURNING widget_id,site_id,public_key,status,max_results,created_at
)
SELECT u.widget_id,u.site_id,o.user_id,u.public_key,o.origin,o.host,u.status,u.max_results,u.created_at
FROM upserted u JOIN owned o ON o.site_id=u.site_id`,userID,siteID,key).
		Scan(&out.WidgetID,&out.SiteID,&out.OwnerUserID,&out.PublicKey,&out.Origin,&out.Host,&out.Status,&out.MaxResults,&out.CreatedAt)
	if errors.Is(err,pgx.ErrNoRows){return Config{},ErrForbidden};if err!=nil{return Config{},err};return out,nil
}

func (r *Repository) Rotate(ctx context.Context,userID,siteID int64)(Config,error){
	if r==nil||r.db==nil||userID<=0||siteID<=0{return Config{},ErrInvalid}
	key,err:=newPublicKey();if err!=nil{return Config{},err};var out Config
	err=r.db.QueryRow(ctx,`WITH owned AS (
 SELECT s.site_id,s.user_id,s.origin,s.host FROM webmaster_sites s JOIN domains d ON d.domain_id=s.domain_id
 WHERE s.site_id=$2 AND s.user_id=$1 AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK'
), changed AS (
 UPDATE site_search_widgets w SET public_key=$3,status='ACTIVE',rotated_at=now(),revoked_at=NULL,updated_at=now()
 FROM owned o WHERE w.site_id=o.site_id
 RETURNING w.widget_id,w.site_id,w.public_key,w.status,w.max_results,w.created_at
)
SELECT c.widget_id,c.site_id,o.user_id,c.public_key,o.origin,o.host,c.status,c.max_results,c.created_at FROM changed c JOIN owned o ON o.site_id=c.site_id`,userID,siteID,key).
		Scan(&out.WidgetID,&out.SiteID,&out.OwnerUserID,&out.PublicKey,&out.Origin,&out.Host,&out.Status,&out.MaxResults,&out.CreatedAt)
	if errors.Is(err,pgx.ErrNoRows){return Config{},ErrNotFound};if err!=nil{return Config{},err};return out,nil
}

func (r *Repository) Revoke(ctx context.Context,userID,siteID int64)error{
	if r==nil||r.db==nil||userID<=0||siteID<=0{return ErrInvalid}
	tag,err:=r.db.Exec(ctx,`UPDATE site_search_widgets w SET status='REVOKED',revoked_at=now(),updated_at=now()
FROM webmaster_sites s WHERE w.site_id=$2 AND s.site_id=w.site_id AND s.user_id=$1`,userID,siteID)
	if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound};return nil
}

func (r *Repository) Owned(ctx context.Context,userID,siteID int64)(Config,error){
	var out Config
	err:=r.db.QueryRow(ctx,`SELECT w.widget_id,w.site_id,s.user_id,w.public_key,s.origin,s.host,w.status,w.max_results,w.created_at
FROM site_search_widgets w JOIN webmaster_sites s ON s.site_id=w.site_id WHERE s.user_id=$1 AND s.site_id=$2`,userID,siteID).
		Scan(&out.WidgetID,&out.SiteID,&out.OwnerUserID,&out.PublicKey,&out.Origin,&out.Host,&out.Status,&out.MaxResults,&out.CreatedAt)
	if errors.Is(err,pgx.ErrNoRows){return Config{},ErrNotFound};return out,err
}

func (r *Repository) ResolvePublic(ctx context.Context,key string)(Config,error){
	key=strings.TrimSpace(key);if r==nil||r.db==nil||key==""||len(key)>96{return Config{},ErrInvalid};var out Config
	err:=r.db.QueryRow(ctx,`SELECT w.widget_id,w.site_id,s.user_id,w.public_key,s.origin,s.host,w.status,w.max_results,w.created_at
FROM site_search_widgets w JOIN webmaster_sites s ON s.site_id=w.site_id JOIN domains d ON d.domain_id=s.domain_id
WHERE w.public_key=$1 AND w.status='ACTIVE' AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK'`,key).
		Scan(&out.WidgetID,&out.SiteID,&out.OwnerUserID,&out.PublicKey,&out.Origin,&out.Host,&out.Status,&out.MaxResults,&out.CreatedAt)
	if errors.Is(err,pgx.ErrNoRows){return Config{},ErrNotFound};return out,err
}

func (r *Repository) RecordUsage(ctx context.Context,widgetID int64,resultCount int)error{
	if widgetID<=0{return ErrInvalid};if resultCount<0{resultCount=0};zero:=0;if resultCount==0{zero=1}
	_,err:=r.db.Exec(ctx,`INSERT INTO site_search_usage_daily(widget_id,day,requests,results,zero_results) VALUES($1,CURRENT_DATE,1,$2,$3)
ON CONFLICT(widget_id,day) DO UPDATE SET requests=site_search_usage_daily.requests+1,results=site_search_usage_daily.results+EXCLUDED.results,zero_results=site_search_usage_daily.zero_results+EXCLUDED.zero_results`,widgetID,resultCount,zero);return err
}
