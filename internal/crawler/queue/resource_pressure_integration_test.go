//go:build integration

package queue

import (
	"context"
	"testing"
	"time"
)

func TestLeaseStopsOnCriticalOrStalePressure(t *testing.T){
	pool:=integrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	domainID,urlID:=createDomainAndURL(t,pool,999)
	if _,err:=repo.Enqueue(ctx,urlID,domainID,1,1,time.Now());err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`UPDATE system_settings SET value='{"state":"CRITICAL"}'::jsonb,updated_at=now() WHERE key='resource_pressure'`);err!=nil{t.Fatal(err)}
	tasks,err:=repo.Lease(ctx,"critical-worker",1,30);if err!=nil{t.Fatal(err)};if len(tasks)!=0{t.Fatalf("critical pressure leased %+v",tasks)}
	if _,err:=pool.Exec(ctx,`UPDATE system_settings SET value='{"state":"NORMAL"}'::jsonb,updated_at=now()-interval '2 minutes' WHERE key='resource_pressure'`);err!=nil{t.Fatal(err)}
	tasks,err=repo.Lease(ctx,"stale-worker",1,30);if err!=nil{t.Fatal(err)};if len(tasks)!=0{t.Fatalf("stale pressure leased %+v",tasks)}
}
