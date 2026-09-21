package referral

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid = errors.New("invalid referral input")
	ErrNotFound = errors.New("referral not found")
	ErrExpired = errors.New("attribution expired")
)

var campaignPattern=regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Referral struct{ID int64 `json:"referral_id"`;Code string `json:"code"`;Campaign string `json:"campaign,omitempty"`;Status string `json:"status"`;ExpiresAt *time.Time `json:"expires_at,omitempty"`}
type Attribution struct{Token string `json:"token"`;ExpiresAt time.Time `json:"expires_at"`}
type Daily struct{Day time.Time `json:"day"`;Code string `json:"code,omitempty"`;Campaign string `json:"campaign,omitempty"`;Flow string `json:"flow"`;Starts int64 `json:"starts"`;Conversions int64 `json:"conversions"`}

func random(prefix string,n int)(string,error){raw:=make([]byte,n);if _,err:=rand.Read(raw);err!=nil{return "",err};return prefix+base64.RawURLEncoding.EncodeToString(raw),nil}
func validFlow(v string)bool{switch v{case "WEBMASTER_REGISTER","SITE_VERIFY","WIDGET_ENABLE","AGENCY_CREATE","ORG_CLAIM":return true};return false}

func (r *Repository) Create(ctx context.Context,userID int64,campaign string,ttl time.Duration)(Referral,error){
	campaign=strings.TrimSpace(campaign);if r==nil||r.db==nil||userID<=0||(campaign!=""&&!campaignPattern.MatchString(campaign)){return Referral{},ErrInvalid};if ttl<0||ttl>365*24*time.Hour{return Referral{},ErrInvalid}
	code,err:=random("ref_",18);if err!=nil{return Referral{},err};var expires any;if ttl>0{expires=time.Now().UTC().Add(ttl)}
	var out Referral;err=r.db.QueryRow(ctx,`INSERT INTO growth_referrals(owner_user_id,code,campaign_key,expires_at) VALUES($1,$2,NULLIF($3,''),$4) RETURNING referral_id,code,COALESCE(campaign_key,''),status,expires_at`,userID,code,campaign,expires).Scan(&out.ID,&out.Code,&out.Campaign,&out.Status,&out.ExpiresAt);return out,err
}

func (r *Repository) Revoke(ctx context.Context,userID,referralID int64)error{if r==nil||r.db==nil||userID<=0||referralID<=0{return ErrInvalid};tag,err:=r.db.Exec(ctx,`UPDATE growth_referrals SET status='REVOKED',revoked_at=now() WHERE referral_id=$1 AND owner_user_id=$2 AND status='ACTIVE'`,referralID,userID);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound};return nil}

func (r *Repository) Start(ctx context.Context,code,campaign,flow,path string)(Attribution,error){
	code=strings.TrimSpace(code);campaign=strings.TrimSpace(campaign);flow=strings.ToUpper(strings.TrimSpace(flow));path=strings.TrimSpace(path)
	if r==nil||r.db==nil||!validFlow(flow)||(campaign!=""&&!campaignPattern.MatchString(campaign))||len(path)>256||(path!=""&&!strings.HasPrefix(path,"/")){return Attribution{},ErrInvalid}
	var referralID *int64;var ownerID *int64;var storedCampaign string
	if code!=""{var rid,oid int64;err:=r.db.QueryRow(ctx,`SELECT referral_id,owner_user_id,COALESCE(campaign_key,'') FROM growth_referrals WHERE code=$1 AND status='ACTIVE' AND (expires_at IS NULL OR expires_at>now())`,code).Scan(&rid,&oid,&storedCampaign);if errors.Is(err,pgx.ErrNoRows){return Attribution{},ErrNotFound};if err!=nil{return Attribution{},err};referralID=&rid;ownerID=&oid;if campaign==""{campaign=storedCampaign}}
	if code==""&&campaign==""{return Attribution{},ErrInvalid}
	token,err:=random("attr_",24);if err!=nil{return Attribution{},err};sum:=sha256.Sum256([]byte(token));expires:=time.Now().UTC().Add(24*time.Hour)
	tx,err:=r.db.Begin(ctx);if err!=nil{return Attribution{},err};defer func(){_=tx.Rollback(ctx)}()
	var attributionID int64;if err=tx.QueryRow(ctx,`INSERT INTO growth_attribution_sessions(token_hash,referral_id,campaign_key,flow,landing_path,expires_at) VALUES($1,$2,NULLIF($3,''),$4,NULLIF($5,''),$6) RETURNING attribution_id`,sum[:],referralID,campaign,flow,path,expires).Scan(&attributionID);err!=nil{return Attribution{},err}
	_,err=tx.Exec(ctx,`INSERT INTO growth_attribution_daily(owner_user_id,referral_id,campaign_key,flow,day,starts) VALUES($1,$2,$3,$4,CURRENT_DATE,1) ON CONFLICT(owner_user_id,referral_id,campaign_key,flow,day) DO UPDATE SET starts=growth_attribution_daily.starts+1`,ownerID,referralID,campaign,flow);if err!=nil{return Attribution{},err};if err=tx.Commit(ctx);err!=nil{return Attribution{},err};_ = attributionID;return Attribution{Token:token,ExpiresAt:expires},nil
}

func (r *Repository) Complete(ctx context.Context,token string)(bool,error){
	token=strings.TrimSpace(token);if r==nil||r.db==nil||token==""||len(token)>128{return false,ErrInvalid};sum:=sha256.Sum256([]byte(token));tx,err:=r.db.Begin(ctx);if err!=nil{return false,err};defer func(){_=tx.Rollback(ctx)}()
	var id int64;var referralID *int64;var campaign,flow string;var expires time.Time
	err=tx.QueryRow(ctx,`SELECT attribution_id,referral_id,COALESCE(campaign_key,''),flow,expires_at FROM growth_attribution_sessions WHERE token_hash=$1 AND converted_at IS NULL FOR UPDATE`,sum[:]).Scan(&id,&referralID,&campaign,&flow,&expires);if errors.Is(err,pgx.ErrNoRows){return false,ErrNotFound};if err!=nil{return false,err};if time.Now().After(expires){return false,ErrExpired}
	if _,err=tx.Exec(ctx,`UPDATE growth_attribution_sessions SET converted_at=now() WHERE attribution_id=$1 AND converted_at IS NULL`,id);err!=nil{return false,err}
	var ownerID *int64;if referralID!=nil{var oid int64;if err=tx.QueryRow(ctx,`SELECT owner_user_id FROM growth_referrals WHERE referral_id=$1`,*referralID).Scan(&oid);err!=nil{return false,err};ownerID=&oid}
	_,err=tx.Exec(ctx,`INSERT INTO growth_attribution_daily(owner_user_id,referral_id,campaign_key,flow,day,conversions) VALUES($1,$2,$3,$4,CURRENT_DATE,1) ON CONFLICT(owner_user_id,referral_id,campaign_key,flow,day) DO UPDATE SET conversions=growth_attribution_daily.conversions+1`,ownerID,referralID,campaign,flow);if err!=nil{return false,err};if err=tx.Commit(ctx);err!=nil{return false,err};return true,nil
}

func (r *Repository) DailyForUser(ctx context.Context,userID int64,days int)([]Daily,error){
	if r==nil||r.db==nil||userID<=0{return nil,ErrInvalid};if days<=0{days=30};if days>90{days=90}
	rows,err:=r.db.Query(ctx,`SELECT d.day,r.code,d.campaign_key,d.flow,d.starts,d.conversions FROM growth_attribution_daily d JOIN growth_referrals r ON r.referral_id=d.referral_id WHERE d.owner_user_id=$1 AND d.day>=CURRENT_DATE-$2::int ORDER BY d.day DESC,r.referral_id,d.flow`,userID,days);if err!=nil{return nil,err};defer rows.Close();out:=[]Daily{};for rows.Next(){var v Daily;if err:=rows.Scan(&v.Day,&v.Code,&v.Campaign,&v.Flow,&v.Starts,&v.Conversions);err!=nil{return nil,err};out=append(out,v)};return out,rows.Err()
}
