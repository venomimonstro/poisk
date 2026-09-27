package main

import (
	"net/url"
	"strings"
	"testing"
)

func setFreshEnv(t *testing.T,db string){
	t.Helper();t.Setenv("READINESS_FRESH_POSTGRES_HOST","127.0.0.1");t.Setenv("READINESS_FRESH_POSTGRES_PORT","5432");t.Setenv("READINESS_FRESH_POSTGRES_DB",db);t.Setenv("READINESS_FRESH_POSTGRES_USER","poisk_test");t.Setenv("READINESS_FRESH_POSTGRES_PASSWORD","secret-value");t.Setenv("READINESS_FRESH_POSTGRES_SSLMODE","disable");t.Setenv("READINESS_FRESH_CONFIRM","MIGRATE:"+db)
}

func TestFreshInstallDSNRequiresDisposableDatabaseName(t *testing.T){
	setFreshEnv(t,"poisk")
	if _,_,err:=freshInstallDSN();err==nil{t.Fatal("expected production-like database name rejection")}
}

func TestFreshInstallDSNAcceptsReadinessDatabase(t *testing.T){
	setFreshEnv(t,"poisk_readiness_test")
	dsn,name,err:=freshInstallDSN();if err!=nil{t.Fatal(err)};if name!="poisk_readiness_test"{t.Fatalf("name=%q",name)}
	u,err:=url.Parse(dsn);if err!=nil{t.Fatal(err)};if u.Hostname()!="127.0.0.1"||strings.TrimPrefix(u.Path,"/")!=name{t.Fatalf("dsn target=%s%s",u.Host,u.Path)};if u.Query().Get("sslmode")!="disable"{t.Fatalf("sslmode=%q",u.Query().Get("sslmode"))}
}

func TestFreshInstallDSNRejectsMissingPassword(t *testing.T){
	setFreshEnv(t,"poisk_readiness_test");t.Setenv("READINESS_FRESH_POSTGRES_PASSWORD","")
	if _,_,err:=freshInstallDSN();err==nil{t.Fatal("expected missing password rejection")}
}

func TestFreshInstallDSNRejectsWrongConfirmation(t *testing.T){
	setFreshEnv(t,"poisk_readiness_test");t.Setenv("READINESS_FRESH_CONFIRM","MIGRATE:other")
	if _,_,err:=freshInstallDSN();err==nil{t.Fatal("expected explicit confirmation rejection")}
}

func TestSameInt64s(t *testing.T){
	if !sameInt64s([]int64{1,2,4},[]int64{1,2,4}){t.Fatal("equal sets should match")}
	if sameInt64s([]int64{1,2},[]int64{1,3}){t.Fatal("different versions must not match")}
}
