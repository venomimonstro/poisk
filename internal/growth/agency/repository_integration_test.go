//go:build integration

package agency

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func agencyDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")}
	pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)};t.Cleanup(pool.Close)
	_,err=pool.Exec(context.Background(),`TRUNCATE TABLE agency_audit_events,agency_site_access,agency_members,agencies,webmaster_sites,webmaster_sessions,webmaster_users,domains RESTART IDENTITY CASCADE`);if err!=nil{t.Fatal(err)}
	return pool
}

func makeUserSite(t *testing.T,p *pgxpool.Pool,email,host string)(int64,int64){
	t.Helper();ctx:=context.Background();var uid,did,sid int64
	if err:=p.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash) VALUES($1,'x') RETURNING user_id`,email).Scan(&uid);err!=nil{t.Fatal(err)}
	if err:=p.QueryRow(ctx,`INSERT INTO domains(host) VALUES($1) RETURNING domain_id`,host).Scan(&did);err!=nil{t.Fatal(err)}
	if err:=p.QueryRow(ctx,`INSERT INTO webmaster_sites(user_id,domain_id,origin,host,status,verified_at,verification_method) VALUES($1,$2,$3,$4,'VERIFIED',now(),'META_TAG') RETURNING site_id`,uid,did,"https://"+host,host).Scan(&sid);err!=nil{t.Fatal(err)}
	return uid,sid
}

func TestAgencyDelegationTenantIsolation(t *testing.T){
	p:=agencyDB(t);repo:=NewRepository(p);ctx:=context.Background()
	agencyOwner,_:=makeUserSite(t,p,"agency@example.test","agency.test")
	client,clientSite:=makeUserSite(t,p,"client@example.test","client.test")
	intruder,intruderSite:=makeUserSite(t,p,"intruder@example.test","intruder.test")
	a,err:=repo.Create(ctx,agencyOwner,"Agency");if err!=nil{t.Fatal(err)}
	if err:=repo.GrantSite(ctx,intruder,a.ID,clientSite,"MANAGE");!errors.Is(err,ErrForbidden){t.Fatalf("intruder grant err=%v",err)}
	if err:=repo.GrantSite(ctx,client,a.ID,clientSite,"MANAGE");err!=nil{t.Fatal(err)}
	sites,err:=repo.ListSites(ctx,agencyOwner,a.ID);if err!=nil{t.Fatal(err)};if len(sites)!=1||sites[0].SiteID!=clientSite||sites[0].Permission!="MANAGE"{t.Fatalf("sites=%+v",sites)}
	if sites,err:=repo.ListSites(ctx,intruder,a.ID);err!=nil||len(sites)!=0{t.Fatalf("intruder sites=%+v err=%v",sites,err)}
	if err:=repo.RevokeSite(ctx,intruder,a.ID,clientSite);!errors.Is(err,ErrForbidden){t.Fatalf("intruder revoke err=%v",err)}
	if err:=repo.RevokeSite(ctx,client,a.ID,clientSite);err!=nil{t.Fatal(err)}
	if sites,err:=repo.ListSites(ctx,agencyOwner,a.ID);err!=nil||len(sites)!=0{t.Fatalf("after revoke sites=%+v err=%v",sites,err)}
	_ = intruderSite
}
