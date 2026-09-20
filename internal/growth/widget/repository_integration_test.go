//go:build integration

package widget

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func widgetDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err:=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)};t.Cleanup(pool.Close);return pool
}
func makeVerifiedSite(t *testing.T,pool *pgxpool.Pool,suffix string)(userID,siteID int64){
	t.Helper();ctx:=context.Background();stamp:=time.Now().UnixNano();email:=fmt.Sprintf("widget-%s-%d@example.test",suffix,stamp);host:=fmt.Sprintf("widget-%s-%d.example.test",suffix,stamp)
	if err:=pool.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash) VALUES($1,'x') RETURNING user_id`,email).Scan(&userID);err!=nil{t.Fatal(err)}
	var domainID int64;if err:=pool.QueryRow(ctx,`INSERT INTO domains(host) VALUES($1) RETURNING domain_id`,host).Scan(&domainID);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`INSERT INTO webmaster_sites(user_id,domain_id,origin,host,status,verified_at,verification_method) VALUES($1,$2,$3,$4,'VERIFIED',now(),'DNS_TXT') RETURNING site_id`,userID,domainID,"https://"+host,host).Scan(&siteID);err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=pool.Exec(context.Background(),`DELETE FROM webmaster_users WHERE user_id=$1`,userID);_,_=pool.Exec(context.Background(),`DELETE FROM domains WHERE domain_id=$1`,domainID)})
	return
}

func TestWidgetKeyRevocationAndTenantIsolation(t *testing.T){
	pool:=widgetDB(t);repo:=NewRepository(pool);ctx:=context.Background();owner,site:=makeVerifiedSite(t,pool,"owner");other,_:=makeVerifiedSite(t,pool,"other")
	cfg,err:=repo.Ensure(ctx,owner,site);if err!=nil{t.Fatal(err)};if _,err:=repo.ResolvePublic(ctx,cfg.PublicKey);err!=nil{t.Fatalf("public key unresolved: %v",err)}
	if _,err:=repo.Owned(ctx,other,site);!errors.Is(err,ErrNotFound){t.Fatalf("foreign owner read widget: %v",err)}
	if err:=repo.Revoke(ctx,other,site);!errors.Is(err,ErrNotFound){t.Fatalf("foreign owner revoked widget: %v",err)}
	if err:=repo.Revoke(ctx,owner,site);err!=nil{t.Fatal(err)}
	if _,err:=repo.ResolvePublic(ctx,cfg.PublicKey);!errors.Is(err,ErrNotFound){t.Fatalf("revoked key still valid: %v",err)}
	reenabled,err:=repo.Ensure(ctx,owner,site);if err!=nil{t.Fatal(err)};if reenabled.PublicKey==cfg.PublicKey{t.Fatal("revoked key was re-enabled instead of rotated")}
	if _,err:=repo.ResolvePublic(ctx,reenabled.PublicKey);err!=nil{t.Fatal(err)}
}
