//go:build integration

package outbox

import (
	"context"
	"testing"
	"time"
)

func TestLeaseStopsOnCriticalOrStalePressure(t *testing.T){
	pool:=integrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	if _,err:=repo.Enqueue(ctx,"WEB_DOCUMENT",8080,1,"UPSERT",time.Now());err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`UPDATE system_settings SET value='{"state":"CRITICAL"}'::jsonb,updated_at=now() WHERE key='resource_pressure'`);err!=nil{t.Fatal(err)}
	events,err:=repo.Lease(ctx,"critical-indexer",1,30);if err!=nil{t.Fatal(err)};if len(events)!=0{t.Fatalf("critical pressure leased %+v",events)}
	if _,err:=pool.Exec(ctx,`UPDATE system_settings SET value='{"state":"NORMAL"}'::jsonb,updated_at=now()-interval '2 minutes' WHERE key='resource_pressure'`);err!=nil{t.Fatal(err)}
	events,err=repo.Lease(ctx,"stale-indexer",1,30);if err!=nil{t.Fatal(err)};if len(events)!=0{t.Fatalf("stale pressure leased %+v",events)}
}
