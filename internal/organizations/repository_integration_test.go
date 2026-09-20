//go:build integration

package organizations

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func organizationIntegrationDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required for integration tests")}
	pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)}
	if err:=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)}
	t.Cleanup(pool.Close)
	_,err=pool.Exec(context.Background(),`TRUNCATE TABLE organization_import_events,organization_merge_review,organization_import_plans,
organization_source_links,organizations,organization_staging_rows,organization_import_batches,organization_sources,index_outbox RESTART IDENTITY CASCADE`)
	if err!=nil{t.Fatalf("reset organization fixture: %v",err)}
	return pool
}

func stageOrganization(t *testing.T,repo *Repository,batchID,rowNumber int64,row SourceRow){
	t.Helper();normalized,err:=NormalizeSourceRow(row);if err!=nil{t.Fatal(err)}
	if _,err:=repo.StageValid(context.Background(),batchID,rowNumber,normalized);err!=nil{t.Fatal(err)}
}

func TestDryRunPlansWithoutCanonicalWrites(t *testing.T){
	pool:=organizationIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	if err:=repo.EnsureSource(ctx,"catalog","Catalog",80);err!=nil{t.Fatal(err)}
	batch,err:=repo.CreateBatch(ctx,"catalog","dry-1","DRY_RUN");if err!=nil{t.Fatal(err)}
	stageOrganization(t,repo,batch.ID,1,SourceRow{SourceRecordID:"1",Name:"Кофейня",Phone:"89991234567"})
	if err:=repo.FinishStaging(ctx,batch.ID);err!=nil{t.Fatal(err)}
	if err:=(Planner{Store:repo}).PlanBatch(ctx,batch.ID);err!=nil{t.Fatal(err)}
	var places int;if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organizations`).Scan(&places);err!=nil{t.Fatal(err)}
	if places!=0{t.Fatalf("dry run wrote %d canonical places",places)}
	summary,err:=repo.Summary(ctx,batch.ID);if err!=nil{t.Fatal(err)}
	if summary.Creates!=1||summary.Batch.Status!="PLANNED"||summary.Batch.Mode!="DRY_RUN"{t.Fatalf("summary=%+v",summary)}
}

func TestApplyResumesAfterLeaseExpiryWithoutDuplicates(t *testing.T){
	pool:=organizationIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	if err:=repo.EnsureSource(ctx,"catalog","Catalog",80);err!=nil{t.Fatal(err)}
	batch,err:=repo.CreateBatch(ctx,"catalog","apply-1","APPLY");if err!=nil{t.Fatal(err)}
	stageOrganization(t,repo,batch.ID,1,SourceRow{SourceRecordID:"1",Name:"Аптека Один",Phone:"89991234567"})
	stageOrganization(t,repo,batch.ID,2,SourceRow{SourceRecordID:"2",Name:"Аптека Два",Phone:"89997654321"})
	if err:=repo.FinishStaging(ctx,batch.ID);err!=nil{t.Fatal(err)}
	if err:=(Planner{Store:repo}).PlanBatch(ctx,batch.ID);err!=nil{t.Fatal(err)}
	leased,err:=repo.LeaseReadyApply(ctx,"worker-a",time.Minute);if err!=nil{t.Fatal(err)}
	if leased.ID!=batch.ID{t.Fatalf("leased=%+v",leased)}
	done,err:=repo.ApplyNext(ctx,batch.ID,"worker-a",time.Minute);if err!=nil||done{t.Fatalf("done=%v err=%v",done,err)}
	var places,events int
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organizations`).Scan(&places);err!=nil{t.Fatal(err)}
	if places!=1{t.Fatalf("places after first apply=%d",places)}
	if _,err:=pool.Exec(ctx,`UPDATE organization_import_batches SET lease_until=now()-interval '1 second' WHERE batch_id=$1`,batch.ID);err!=nil{t.Fatal(err)}
	if n,err:=repo.RequeueExpiredApply(ctx);err!=nil||n!=1{t.Fatalf("requeue n=%d err=%v",n,err)}
	if _,err:=repo.LeaseReadyApply(ctx,"worker-b",time.Minute);err!=nil{t.Fatal(err)}
	done,err=repo.ApplyNext(ctx,batch.ID,"worker-b",time.Minute);if err!=nil||done{t.Fatalf("second done=%v err=%v",done,err)}
	done,err=repo.ApplyNext(ctx,batch.ID,"worker-b",time.Minute);if err!=nil||!done{t.Fatalf("finish done=%v err=%v",done,err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM organizations`).Scan(&places);err!=nil{t.Fatal(err)}
	if places!=2{t.Fatalf("places=%d",places)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM index_outbox WHERE entity_type='ORGANIZATION' AND operation='UPSERT'`).Scan(&events);err!=nil{t.Fatal(err)}
	if events!=2{t.Fatalf("organization outbox events=%d",events)}
	summary,err:=repo.Summary(ctx,batch.ID);if err!=nil{t.Fatal(err)}
	if summary.Batch.Status!="DONE"||summary.Applied!=2{t.Fatalf("summary=%+v",summary)}
}

func TestRepeatedSourceIdentityBecomesNoop(t *testing.T){
	pool:=organizationIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	if err:=repo.EnsureSource(ctx,"catalog","Catalog",80);err!=nil{t.Fatal(err)}
	row:=SourceRow{SourceRecordID:"same",Name:"Магазин",Website:"example.org"}
	for i,key:=range []string{"first","second"}{
		batch,err:=repo.CreateBatch(ctx,"catalog",key,"APPLY");if err!=nil{t.Fatal(err)}
		stageOrganization(t,repo,batch.ID,1,row)
		if err:=repo.FinishStaging(ctx,batch.ID);err!=nil{t.Fatal(err)}
		if err:=(Planner{Store:repo}).PlanBatch(ctx,batch.ID);err!=nil{t.Fatal(err)}
		if i==1{summary,err:=repo.Summary(ctx,batch.ID);if err!=nil{t.Fatal(err)};if summary.Noops!=1{t.Fatalf("summary=%+v",summary)}}
		if _,err:=repo.LeaseReadyApply(ctx,"worker",time.Minute);err!=nil{t.Fatal(err)}
		if _,err:=repo.ApplyNext(ctx,batch.ID,"worker",time.Minute);err!=nil{t.Fatal(err)}
		if done,err:=repo.ApplyNext(ctx,batch.ID,"worker",time.Minute);err!=nil||!done{t.Fatalf("done=%v err=%v",done,err)}
	}
	var places,links,events int
	_ = pool.QueryRow(ctx,`SELECT count(*) FROM organizations`).Scan(&places)
	_ = pool.QueryRow(ctx,`SELECT count(*) FROM organization_source_links`).Scan(&links)
	_ = pool.QueryRow(ctx,`SELECT count(*) FROM index_outbox WHERE entity_type='ORGANIZATION'`).Scan(&events)
	if places!=1||links!=1||events!=1{t.Fatalf("places=%d links=%d events=%d",places,links,events)}
}

func TestAmbiguousPlanBlocksApplyUntilReviewResolved(t *testing.T){
	pool:=organizationIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	if err:=repo.EnsureSource(ctx,"catalog","Catalog",80);err!=nil{t.Fatal(err)}
	var first,second int64
	if err:=pool.QueryRow(ctx,`INSERT INTO organizations(name,normalized_name,phone,source_count) VALUES('Кафе','кафе','+79990000001',0) RETURNING place_id`).Scan(&first);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`INSERT INTO organizations(name,normalized_name,website,source_count) VALUES('Кафе','кафе','https://cafe.test/',0) RETURNING place_id`).Scan(&second);err!=nil{t.Fatal(err)}
	batch,err:=repo.CreateBatch(ctx,"catalog","ambiguous","APPLY");if err!=nil{t.Fatal(err)}
	stageOrganization(t,repo,batch.ID,1,SourceRow{SourceRecordID:"a",Name:"Кафе",Phone:"89990000001",Website:"cafe.test"})
	if err:=repo.FinishStaging(ctx,batch.ID);err!=nil{t.Fatal(err)}
	if err:=(Planner{Store:repo}).PlanBatch(ctx,batch.ID);err!=nil{t.Fatal(err)}
	if _,err:=repo.LeaseReadyApply(ctx,"worker",time.Minute);!errors.Is(err,ErrImportNotFound){t.Fatalf("ambiguous batch leased: %v",err)}
	var stagingID int64;if err:=pool.QueryRow(ctx,`SELECT staging_id FROM organization_staging_rows WHERE batch_id=$1`,batch.ID).Scan(&stagingID);err!=nil{t.Fatal(err)}
	if err:=repo.ResolveReview(ctx,stagingID,"MERGE",&first,"tester");err!=nil{t.Fatal(err)}
	if _,err:=repo.LeaseReadyApply(ctx,"worker",time.Minute);err!=nil{t.Fatal(err)}
	if _,err:=repo.ApplyNext(ctx,batch.ID,"worker",time.Minute);err!=nil{t.Fatal(err)}
	_ = second
}
