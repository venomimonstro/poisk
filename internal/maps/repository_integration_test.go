//go:build integration

package maps

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func mapIntegrationDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required for integration tests")}
	pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)}
	if err:=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)}
	t.Cleanup(pool.Close)
	_,err=pool.Exec(context.Background(),`TRUNCATE TABLE map_version_events,map_state,map_versions RESTART IDENTITY CASCADE; INSERT INTO map_state(singleton) VALUES(TRUE);`)
	if err!=nil{t.Fatalf("reset map fixture: %v",err)}
	return pool
}

func TestRepositoryActivateAndRollbackAtomically(t *testing.T){
	pool:=mapIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	_,base:=writeMapFixture(t)
	v1:=base;v1.Version="v1";v1.PMTilesPath="tiles/v1.pmtiles";v1.StylePath="styles/v1.json"
	v2:=base;v2.Version="v2";v2.PMTilesPath="tiles/v2.pmtiles";v2.StylePath="styles/v2.json"
	if err:=repo.Register(ctx,v1,"test");err!=nil{t.Fatal(err)}
	if err:=repo.Register(ctx,v2,"test");err!=nil{t.Fatal(err)}
	if err:=repo.Activate(ctx,"v1","test");err!=nil{t.Fatal(err)}
	if err:=repo.Activate(ctx,"v2","test");err!=nil{t.Fatal(err)}
	state,err:=repo.State(ctx);if err!=nil{t.Fatal(err)}
	if state.ActiveVersion!="v2" || state.PreviousVersion!="v1"{t.Fatalf("state=%+v",state)}
	rolled,err:=repo.Rollback(ctx,"test");if err!=nil{t.Fatal(err)}
	if rolled!="v1"{t.Fatalf("rolled=%q",rolled)}
	state,err=repo.State(ctx);if err!=nil{t.Fatal(err)}
	if state.ActiveVersion!="v1" || state.PreviousVersion!="v2"{t.Fatalf("rollback state=%+v",state)}
	var events int
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM map_version_events`).Scan(&events);err!=nil{t.Fatal(err)}
	if events!=5{t.Fatalf("events=%d",events)}
}

func TestRepositoryRejectsUnknownActivation(t *testing.T){
	pool:=mapIntegrationDB(t);repo:=NewRepository(pool)
	if err:=repo.Activate(context.Background(),"missing","test");err!=ErrMapNotFound{t.Fatalf("err=%v",err)}
}
