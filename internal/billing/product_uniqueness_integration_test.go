//go:build integration

package billing

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentPlansForSameProductOnlyCreateOneSubscription(t *testing.T){
	p:=billingDB(t);repo:=NewRepository(p);ctx:=context.Background();owner:=billingUser(t,p,"owner@example.test")
	if _,err:=p.Exec(ctx,`INSERT INTO billing_plans(product_code,plan_code,version,monthly_price_kopecks,quotas) VALUES
('SITE_SEARCH_PRO','SITE_A',1,149000,'{"requests_month":50000}'::jsonb),
('SITE_SEARCH_PRO','SITE_B',1,499000,'{"requests_month":300000}'::jsonb)`);err!=nil{t.Fatal(err)}
	account,err:=repo.EnsureUserAccount(ctx,owner);if err!=nil{t.Fatal(err)}
	start:=time.Date(2026,9,21,12,0,0,0,time.UTC)
	var success int64;var conflicts int64;var wg sync.WaitGroup;errs:=make(chan error,2)
	for _,code:=range []string{"SITE_A","SITE_B"}{code:=code;wg.Add(1);go func(){defer wg.Done();_,_,err:=repo.CreatePendingSubscription(ctx,account.ID,code,start);switch{case err==nil:atomic.AddInt64(&success,1);case errors.Is(err,ErrConflict):atomic.AddInt64(&conflicts,1);default:errs<-err}}()}
	wg.Wait();close(errs);for err:=range errs{t.Fatal(err)}
	if success!=1||conflicts!=1{t.Fatalf("success=%d conflicts=%d",success,conflicts)}
	var count int64;if err:=p.QueryRow(ctx,`SELECT count(*) FROM billing_subscriptions WHERE account_id=$1 AND product_code='SITE_SEARCH_PRO' AND status IN ('PENDING','ACTIVE','GRACE','PAST_DUE')`,account.ID).Scan(&count);err!=nil{t.Fatal(err)};if count!=1{t.Fatalf("active product subscriptions=%d",count)}
}
