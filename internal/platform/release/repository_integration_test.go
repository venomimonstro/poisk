//go:build integration

package release

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
)

func releaseIntegrationDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required for integration tests")}
	pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err:=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)};t.Cleanup(pool.Close);return pool
}

func TestRollbackCompatibilityRequiresExactDatabaseSchema(t *testing.T){
	pool:=releaseIntegrationDB(t);ctx:=context.Background();repo:=Repository{DB:pool}
	var schema int64;if err:=pool.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&schema);err!=nil{t.Fatal(err)};if schema<2{t.Fatalf("schema=%d",schema)}
	manifest:=Manifest{RequiredSchemaVersion:schema,WebIndexSchema:indexmanticore.WebSchemaVersion,OrganizationIndexSchema:indexmanticore.OrganizationsSchemaVersion,AddressIndexSchema:indexmanticore.AddressesSchemaVersion,BackendImage:"backend:test",FrontendImage:"frontend:test"}
	if err:=repo.ValidateRollbackCompatibility(ctx,manifest);err!=nil{t.Fatalf("exact schema rejected: %v",err)}
	manifest.RequiredSchemaVersion=schema-1
	if err:=repo.ValidateRollbackCompatibility(ctx,manifest);err!=ErrPreflight{t.Fatalf("older schema rollback accepted: %v",err)}
}

func TestRestageInvalidatesPreflightAndAuditsStage(t *testing.T){
	pool:=releaseIntegrationDB(t);ctx:=context.Background();repo:=Repository{DB:pool}
	var schema int64;if err:=pool.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&schema);err!=nil{t.Fatal(err)}
	version:=fmt.Sprintf("integration-%d",time.Now().UnixNano());commit:="0123456789abcdef0123456789abcdef01234567";hash:="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	first,err:=repo.Stage(ctx,version,commit,schema,"",hash,"backend:test-a","frontend:test","integration-test");if err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_,_=pool.Exec(context.Background(),`DELETE FROM release_events WHERE release_id=$1`,first.ID);_,_=pool.Exec(context.Background(),`DELETE FROM app_releases WHERE release_id=$1`,first.ID)})
	if _,err:=repo.Preflight(ctx,version,"integration-test");err!=nil{t.Fatal(err)}
	var before *time.Time;if err:=pool.QueryRow(ctx,`SELECT preflight_at FROM app_releases WHERE release_id=$1`,first.ID).Scan(&before);err!=nil{t.Fatal(err)};if before==nil{t.Fatal("preflight_at not set")}
	second,err:=repo.Stage(ctx,version,commit,schema,"",hash,"backend:test-b","frontend:test","integration-test");if err!=nil{t.Fatal(err)};if second.ID!=first.ID{t.Fatalf("release id changed: %d != %d",second.ID,first.ID)}
	var after *time.Time;if err:=pool.QueryRow(ctx,`SELECT preflight_at FROM app_releases WHERE release_id=$1`,first.ID).Scan(&after);err!=nil{t.Fatal(err)};if after!=nil{t.Fatal("restage did not invalidate preflight")}
	var stageEvents int;if err:=pool.QueryRow(ctx,`SELECT count(*) FROM release_events WHERE release_id=$1 AND action='STAGE'`,first.ID).Scan(&stageEvents);err!=nil{t.Fatal(err)};if stageEvents!=2{t.Fatalf("stage events=%d",stageEvents)}
}
