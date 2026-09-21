//go:build integration

package claim

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func claimDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};p,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=p.Ping(context.Background());err!=nil{p.Close();t.Fatal(err)};t.Cleanup(p.Close)
	_,err=p.Exec(context.Background(),`TRUNCATE TABLE organization_claim_events,organization_claims,organization_web_links,webmaster_sites,webmaster_sessions,webmaster_users,organizations,domains RESTART IDENTITY CASCADE`);if err!=nil{t.Fatal(err)};return p
}
func claimUserSite(t *testing.T,p *pgxpool.Pool,email,host string)(int64,int64,int64){
	t.Helper();ctx:=context.Background();var uid,did,sid int64
	if err:=p.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash) VALUES($1,'x') RETURNING user_id`,email).Scan(&uid);err!=nil{t.Fatal(err)}
	if err:=p.QueryRow(ctx,`INSERT INTO domains(host) VALUES($1) RETURNING domain_id`,host).Scan(&did);err!=nil{t.Fatal(err)}
	if err:=p.QueryRow(ctx,`INSERT INTO webmaster_sites(user_id,domain_id,origin,host,status,verified_at,verification_method) VALUES($1,$2,$3,$4,'VERIFIED',now(),'META_TAG') RETURNING site_id`,uid,did,"https://"+host,host).Scan(&sid);err!=nil{t.Fatal(err)}
	return uid,did,sid
}
func TestClaimRequiresVerifiedExactHostAndProvenance(t *testing.T){
	p:=claimDB(t);repo:=NewRepository(p);ctx:=context.Background();owner,domainID,siteID:=claimUserSite(t,p,"owner@example.test","business.test");intruder,_,intruderSite:=claimUserSite(t,p,"intruder@example.test","other.test")
	var placeID int64;if err:=p.QueryRow(ctx,`INSERT INTO organizations(name,normalized_name,website,status) VALUES('Business','business','https://business.test/','ACTIVE') RETURNING place_id`).Scan(&placeID);err!=nil{t.Fatal(err)}
	if _,err:=repo.Claim(ctx,owner,siteID,placeID);!errors.Is(err,ErrForbidden){t.Fatalf("claim without provenance err=%v",err)}
	if _,err:=p.Exec(ctx,`INSERT INTO organization_web_links(place_id,domain_id,match_type,confidence,evidence) VALUES($1,$2,'WEBSITE_HOST',100,'{}')`,placeID,domainID);err!=nil{t.Fatal(err)}
	if _,err:=repo.Claim(ctx,intruder,intruderSite,placeID);!errors.Is(err,ErrForbidden){t.Fatalf("foreign host claim err=%v",err)}
	c,err:=repo.Claim(ctx,owner,siteID,placeID);if err!=nil{t.Fatal(err)};if c.PlaceID!=placeID||c.SiteID!=siteID||c.ProofHost!="business.test"{t.Fatalf("claim=%+v",c)}
	if _,err:=repo.Claim(ctx,intruder,intruderSite,placeID);!errors.Is(err,ErrForbidden){t.Fatalf("intruder claim after owner err=%v",err)}
	if err:=repo.Revoke(ctx,intruder,placeID);!errors.Is(err,ErrNotFound){t.Fatalf("intruder revoke err=%v",err)}
	if err:=repo.Revoke(ctx,owner,placeID);err!=nil{t.Fatal(err)}
}
