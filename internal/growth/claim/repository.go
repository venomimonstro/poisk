package claim

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("organization claim target not found")
	ErrForbidden = errors.New("organization claim proof failed")
	ErrConflict = errors.New("organization already claimed")
	ErrInvalid = errors.New("invalid organization claim input")
)

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Claim struct{ID int64 `json:"claim_id"`;PlaceID int64 `json:"place_id"`;SiteID int64 `json:"site_id"`;ProofHost string `json:"proof_host"`;Status string `json:"status"`}

func (r *Repository) Claim(ctx context.Context,userID,siteID,placeID int64)(Claim,error){
	if r==nil||r.db==nil||userID<=0||siteID<=0||placeID<=0{return Claim{},ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return Claim{},err};defer func(){_=tx.Rollback(ctx)}()
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock($1)`,placeID);err!=nil{return Claim{},err}
	var domainID int64;var siteHost string
	err=tx.QueryRow(ctx,`SELECT s.domain_id,s.host FROM webmaster_sites s JOIN domains d ON d.domain_id=s.domain_id WHERE s.site_id=$1 AND s.user_id=$2 AND s.status='VERIFIED' AND d.status='ACTIVE' AND d.policy<>'BLOCK'`,siteID,userID).Scan(&domainID,&siteHost)
	if errors.Is(err,pgx.ErrNoRows){return Claim{},ErrForbidden};if err!=nil{return Claim{},err}
	var website,status string
	err=tx.QueryRow(ctx,`SELECT COALESCE(website,''),status FROM organizations WHERE place_id=$1`,placeID).Scan(&website,&status)
	if errors.Is(err,pgx.ErrNoRows){return Claim{},ErrNotFound};if err!=nil{return Claim{},err};if status!="ACTIVE"&&status!="REVIEW"{return Claim{},ErrForbidden}
	orgHost,err:=websiteHost(website);if err!=nil||!strings.EqualFold(orgHost,strings.TrimSuffix(strings.ToLower(siteHost),".")){return Claim{},ErrForbidden}
	var provenance bool
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_web_links WHERE place_id=$1 AND domain_id=$2 AND match_type='WEBSITE_HOST' AND confidence>=100)`,placeID,domainID).Scan(&provenance);err!=nil{return Claim{},err};if !provenance{return Claim{},ErrForbidden}
	var existing Claim;var existingUser int64
	err=tx.QueryRow(ctx,`SELECT claim_id,place_id,site_id,proof_host,status,user_id FROM organization_claims WHERE place_id=$1 AND status='ACTIVE' FOR UPDATE`,placeID).Scan(&existing.ID,&existing.PlaceID,&existing.SiteID,&existing.ProofHost,&existing.Status,&existingUser)
	if err==nil{if existingUser==userID&&existing.SiteID==siteID{return existing,tx.Commit(ctx)};return Claim{},ErrConflict};if !errors.Is(err,pgx.ErrNoRows){return Claim{},err}
	var out Claim
	err=tx.QueryRow(ctx,`INSERT INTO organization_claims(place_id,user_id,site_id,proof_type,proof_host,status,revoked_at,updated_at) VALUES($1,$2,$3,'VERIFIED_WEBSITE_HOST',$4,'ACTIVE',NULL,now()) ON CONFLICT(place_id,user_id,site_id) DO UPDATE SET proof_type='VERIFIED_WEBSITE_HOST',proof_host=EXCLUDED.proof_host,status='ACTIVE',revoked_at=NULL,updated_at=now() RETURNING claim_id,place_id,site_id,proof_host,status`,placeID,userID,siteID,orgHost).Scan(&out.ID,&out.PlaceID,&out.SiteID,&out.ProofHost,&out.Status);if err!=nil{return Claim{},err}
	evidence,_:=json.Marshal(map[string]any{"proof_type":"VERIFIED_WEBSITE_HOST","host":orgHost,"domain_id":domainID,"provenance":"WEBSITE_HOST"})
	if _,err=tx.Exec(ctx,`INSERT INTO organization_claim_events(claim_id,place_id,user_id,site_id,action,evidence) VALUES($1,$2,$3,$4,'CLAIM',$5::jsonb)`,out.ID,placeID,userID,siteID,string(evidence));err!=nil{return Claim{},err}
	if err=tx.Commit(ctx);err!=nil{return Claim{},err};return out,nil
}

func (r *Repository) Revoke(ctx context.Context,userID,placeID int64)error{
	if r==nil||r.db==nil||userID<=0||placeID<=0{return ErrInvalid};tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var claimID,siteID int64;var host string
	err=tx.QueryRow(ctx,`UPDATE organization_claims SET status='REVOKED',revoked_at=now(),updated_at=now() WHERE place_id=$1 AND user_id=$2 AND status='ACTIVE' RETURNING claim_id,site_id,proof_host`,placeID,userID).Scan(&claimID,&siteID,&host);if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err}
	evidence,_:=json.Marshal(map[string]any{"host":host});if _,err=tx.Exec(ctx,`INSERT INTO organization_claim_events(claim_id,place_id,user_id,site_id,action,evidence) VALUES($1,$2,$3,$4,'REVOKE',$5::jsonb)`,claimID,placeID,userID,siteID,string(evidence));err!=nil{return err};return tx.Commit(ctx)
}

func (r *Repository) List(ctx context.Context,userID int64)([]Claim,error){
	if r==nil||r.db==nil||userID<=0{return nil,ErrInvalid};rows,err:=r.db.Query(ctx,`SELECT claim_id,place_id,site_id,proof_host,status FROM organization_claims WHERE user_id=$1 AND status='ACTIVE' ORDER BY claim_id`,userID);if err!=nil{return nil,err};defer rows.Close();out:=[]Claim{};for rows.Next(){var c Claim;if err:=rows.Scan(&c.ID,&c.PlaceID,&c.SiteID,&c.ProofHost,&c.Status);err!=nil{return nil,err};out=append(out,c)};return out,rows.Err()
}

func websiteHost(raw string)(string,error){u,err:=url.Parse(strings.TrimSpace(raw));if err!=nil{return "",err};host:=strings.ToLower(strings.TrimSuffix(u.Hostname(),"."));if host==""{return "",ErrForbidden};return host,nil}
