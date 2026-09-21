//go:build integration

package billing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func billingDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};p,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=p.Ping(context.Background());err!=nil{p.Close();t.Fatal(err)};t.Cleanup(p.Close)
	_,err=p.Exec(context.Background(),`TRUNCATE TABLE billing_usage_periods,billing_ledger_entries,billing_payment_events,billing_invoices,billing_subscriptions,billing_accounts,billing_plans,webmaster_users RESTART IDENTITY CASCADE`);if err!=nil{t.Fatal(err)}
	return p
}
func billingUser(t *testing.T,p *pgxpool.Pool,email string)int64{t.Helper();var id int64;if err:=p.QueryRow(context.Background(),`INSERT INTO webmaster_users(email,password_hash) VALUES($1,'x') RETURNING user_id`,email).Scan(&id);err!=nil{t.Fatal(err)};return id}
func testPlan(t *testing.T,p *pgxpool.Pool,code string,limit int64){t.Helper();_,err:=p.Exec(context.Background(),`INSERT INTO billing_plans(product_code,plan_code,version,monthly_price_kopecks,quotas) VALUES('WEBMASTER_PRO',$1,1,1000,jsonb_build_object('url_requests_month',$2))`,code,limit);if err!=nil{t.Fatal(err)}}

func TestPaymentEventIdempotencyAndAccountIsolation(t *testing.T){
	p:=billingDB(t);repo:=NewRepository(p);ctx:=context.Background();owner:=billingUser(t,p,"owner@example.test");other:=billingUser(t,p,"other@example.test");testPlan(t,p,"WEBMASTER_TEST",5)
	account,err:=repo.EnsureUserAccount(ctx,owner);if err!=nil{t.Fatal(err)}
	if _,err:=repo.AccountForUser(ctx,other,account.ID);!errors.Is(err,ErrForbidden){t.Fatalf("foreign account err=%v",err)}
	now:=time.Date(2026,9,21,12,0,0,0,time.UTC);sub,invoice,err:=repo.CreatePendingSubscription(ctx,account.ID,"WEBMASTER_TEST",now);if err!=nil{t.Fatal(err)};if sub.Status!="PENDING"||invoice.Status!="OPEN"{t.Fatalf("sub=%+v invoice=%+v",sub,invoice)}
	event:=PaymentEvent{Provider:"TEST",ProviderEventID:"evt-1",Type:"PAYMENT_SUCCEEDED",InvoiceID:invoice.ID,AmountKopecks:invoice.AmountKopecks,Currency:"RUB",Payload:[]byte(`{"id":"evt-1"}`),OccurredAt:now}
	first,err:=repo.ApplyPaymentEvent(ctx,event);if err!=nil||!first.Processed||first.SubscriptionStatus!="ACTIVE"{t.Fatalf("first=%+v err=%v",first,err)}
	second,err:=repo.ApplyPaymentEvent(ctx,event);if err!=nil||second.Processed{t.Fatalf("second=%+v err=%v",second,err)}
	mutated:=event;mutated.AmountKopecks++
	if _,err:=repo.ApplyPaymentEvent(ctx,mutated);!errors.Is(err,ErrPaymentMismatch){t.Fatalf("mutated replay err=%v",err)}
	var payments int;if err:=p.QueryRow(ctx,`SELECT count(*) FROM billing_ledger_entries WHERE invoice_id=$1 AND entry_type='PAYMENT'`,invoice.ID).Scan(&payments);err!=nil{t.Fatal(err)};if payments!=1{t.Fatalf("payment ledger entries=%d",payments)}
	ent,err:=repo.Entitlement(ctx,account.ID,"WEBMASTER_PRO",now.Add(time.Hour));if err!=nil||!ent.Active||ent.PlanCode!="WEBMASTER_TEST"{t.Fatalf("ent=%+v err=%v",ent,err)}
}

func TestQuotaConsumeIsRaceSafe(t *testing.T){
	p:=billingDB(t);repo:=NewRepository(p);ctx:=context.Background();owner:=billingUser(t,p,"owner@example.test");testPlan(t,p,"WEBMASTER_QUOTA",5);account,err:=repo.EnsureUserAccount(ctx,owner);if err!=nil{t.Fatal(err)}
	now:=time.Now().UTC();_,invoice,err:=repo.CreatePendingSubscription(ctx,account.ID,"WEBMASTER_QUOTA",now);if err!=nil{t.Fatal(err)}
	_,err=repo.ApplyPaymentEvent(ctx,PaymentEvent{Provider:"TEST",ProviderEventID:"quota-paid",Type:"PAYMENT_SUCCEEDED",InvoiceID:invoice.ID,AmountKopecks:invoice.AmountKopecks,Currency:"RUB",Payload:[]byte("quota"),OccurredAt:now});if err!=nil{t.Fatal(err)}
	var successes int64;var wg sync.WaitGroup;errs:=make(chan error,20)
	for i:=0;i<20;i++{wg.Add(1);go func(i int){defer wg.Done();_,_,err:=repo.Consume(ctx,account.ID,"WEBMASTER_PRO","url_requests_month",1,now.Add(time.Minute));if err==nil{atomic.AddInt64(&successes,1);return};if !errors.Is(err,ErrQuotaExceeded){errs<-fmt.Errorf("worker %d: %w",i,err)}}(i)}
	wg.Wait();close(errs);for err:=range errs{t.Fatal(err)};if successes!=5{t.Fatalf("successes=%d want=5",successes)}
	usage,err:=repo.Usage(ctx,owner,account.ID);if err!=nil{t.Fatal(err)};if usage["url_requests_month"]!=5{t.Fatalf("usage=%v",usage)}
}

func TestLifecycleExpiryRemovesEntitlementWithoutDataDeletion(t *testing.T){
	p:=billingDB(t);repo:=NewRepository(p);ctx:=context.Background();owner:=billingUser(t,p,"owner@example.test");testPlan(t,p,"WEBMASTER_LIFECYCLE",5);account,_:=repo.EnsureUserAccount(ctx,owner);start:=time.Date(2026,1,1,0,0,0,0,time.UTC);_,invoice,err:=repo.CreatePendingSubscription(ctx,account.ID,"WEBMASTER_LIFECYCLE",start);if err!=nil{t.Fatal(err)}
	if _,err=repo.ApplyPaymentEvent(ctx,PaymentEvent{Provider:"TEST",ProviderEventID:"life-paid",Type:"PAYMENT_SUCCEEDED",InvoiceID:invoice.ID,AmountKopecks:invoice.AmountKopecks,Currency:"RUB",Payload:[]byte("life"),OccurredAt:start});err!=nil{t.Fatal(err)}
	if graced,expired,err:=repo.ReconcileLifecycle(ctx,start.AddDate(0,1,1));err!=nil||graced!=1||expired!=0{t.Fatalf("graced=%d expired=%d err=%v",graced,expired,err)}
	ent,err:=repo.Entitlement(ctx,account.ID,"WEBMASTER_PRO",start.AddDate(0,1,1));if err!=nil||!ent.Active{t.Fatalf("grace entitlement=%+v err=%v",ent,err)}
	if _,expired,err:=repo.ReconcileLifecycle(ctx,start.AddDate(0,1,4));err!=nil||expired!=1{t.Fatalf("expired=%d err=%v",expired,err)}
	ent,err=repo.Entitlement(ctx,account.ID,"WEBMASTER_PRO",start.AddDate(0,1,4));if err!=nil||ent.Active{t.Fatalf("expired entitlement=%+v err=%v",ent,err)}
	var users int;if err:=p.QueryRow(ctx,`SELECT count(*) FROM webmaster_users WHERE user_id=$1`,owner).Scan(&users);err!=nil{t.Fatal(err)};if users!=1{t.Fatal("canonical user was deleted")}
}
