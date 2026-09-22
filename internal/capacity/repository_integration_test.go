//go:build integration

package capacity

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCapacitySnapshotAndADRAreImmutable(t *testing.T){
	dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")}
	ctx:=context.Background();pool,err:=pgxpool.New(ctx,dsn);if err!=nil{t.Fatal(err)};defer pool.Close()
	if _,err=pool.Exec(ctx,`TRUNCATE TABLE capacity_adr_decisions,capacity_snapshots,capacity_benchmark_runs RESTART IDENTITY CASCADE`);err!=nil{t.Fatal(err)}
	repo:=NewRepository(pool);runID,err:=repo.StartRun(ctx,"integration","LIVE_READONLY",10_000_000,map[string]any{"test":true});if err!=nil{t.Fatal(err)}
	projection,_:=ProjectStorage(100,10_000_000,StorageSnapshot{DatabaseBytes:1000,ManticoreBytes:2000})
	snapshotID,err:=repo.CompleteRun(ctx,FinalSnapshot{RunID:runID,MeasuredDocuments:100,MeasuredAt:time.Now().UTC(),Workload:map[string]WorkloadMetrics{"search":{Requests:10,QPS:10}},Resources:map[string]any{"server_measurements_provided":false},Queues:map[string]any{},Storage:StorageSnapshot{DatabaseBytes:1000,ManticoreBytes:2000},Projection:projection,Bottlenecks:[]Bottleneck{}});if err!=nil{t.Fatal(err)}
	if snapshotID<=0{t.Fatalf("snapshot=%d",snapshotID)}
	if _,err=pool.Exec(ctx,`UPDATE capacity_snapshots SET measured_documents=101 WHERE snapshot_id=$1`,snapshotID);err==nil{t.Fatal("expected immutable snapshot update rejection")}
	decisionID,markdown,err:=repo.CreateADR(ctx,snapshotID,"STAY_SINGLE_NODE","tester","Measured values remain within the declared test capacity budget.");if err!=nil{t.Fatal(err)}
	if decisionID<=0||markdown==""{t.Fatalf("decision=%d markdown=%q",decisionID,markdown)}
	if _,_,err=repo.CreateADR(ctx,snapshotID,"MOVE_CRAWLER","tester","A second decision for the same immutable snapshot must be rejected.");err==nil{t.Fatal("expected duplicate ADR rejection")}
	if _,err=pool.Exec(ctx,`DELETE FROM capacity_adr_decisions WHERE decision_id=$1`,decisionID);err==nil{t.Fatal("expected immutable ADR delete rejection")}
	items,err:=repo.RecentSnapshots(ctx,10);if err!=nil{t.Fatal(err)};if len(items)!=1||items[0].Decision!="STAY_SINGLE_NODE"{raw,_:=json.Marshal(items);t.Fatalf("items=%s",raw)}
}
