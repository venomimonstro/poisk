package main

import (
	"net/url"
	"testing"
)

func setUpgradeEnv(t *testing.T,db,from string){
	t.Helper();t.Setenv("READINESS_UPGRADE_POSTGRES_HOST","127.0.0.1");t.Setenv("READINESS_UPGRADE_POSTGRES_PORT","5432");t.Setenv("READINESS_UPGRADE_POSTGRES_DB",db);t.Setenv("READINESS_UPGRADE_POSTGRES_USER","poisk_test");t.Setenv("READINESS_UPGRADE_POSTGRES_PASSWORD","secret-value");t.Setenv("READINESS_UPGRADE_POSTGRES_SSLMODE","disable");t.Setenv("READINESS_UPGRADE_FROM_SCHEMA",from);t.Setenv("READINESS_UPGRADE_CONFIRM","UPGRADE:"+db)
}

func TestUpgradeInstallDSNRequiresDisposableDatabaseName(t *testing.T){
	setUpgradeEnv(t,"poisk","60")
	if _,_,_,err:=upgradeInstallDSN();err==nil{t.Fatal("expected production-like database name rejection")}
}

func TestUpgradeInstallDSNAcceptsReadinessCopy(t *testing.T){
	setUpgradeEnv(t,"poisk_upgrade_readiness","60")
	dsn,name,from,err:=upgradeInstallDSN();if err!=nil{t.Fatal(err)};if name!="poisk_upgrade_readiness"||from!=60{t.Fatalf("name=%q from=%d",name,from)}
	u,err:=url.Parse(dsn);if err!=nil{t.Fatal(err)};if u.Hostname()!="127.0.0.1"||u.Query().Get("sslmode")!="disable"{t.Fatalf("unexpected dsn target")}
}

func TestUpgradeInstallDSNRejectsInvalidSchema(t *testing.T){
	setUpgradeEnv(t,"poisk_upgrade_test","latest")
	if _,_,_,err:=upgradeInstallDSN();err==nil{t.Fatal("expected invalid source schema rejection")}
}

func TestUpgradeInstallDSNRejectsWrongConfirmation(t *testing.T){
	setUpgradeEnv(t,"poisk_upgrade_test","60");t.Setenv("READINESS_UPGRADE_CONFIRM","UPGRADE:other")
	if _,_,_,err:=upgradeInstallDSN();err==nil{t.Fatal("expected explicit confirmation rejection")}
}
