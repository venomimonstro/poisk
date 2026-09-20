//go:build integration

package webmaster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	wmauth "github.com/venomimonstro/poisk/internal/webmaster/auth"
)

func webmasterIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn:=os.Getenv("TEST_DATABASE_URL")
	if dsn==""{t.Fatal("TEST_DATABASE_URL is required for integration tests")}
	pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)}
	if err:=pool.Ping(context.Background());err!=nil{pool.Close();t.Fatal(err)}
	t.Cleanup(pool.Close)
	_,err=pool.Exec(context.Background(),`
TRUNCATE TABLE webmaster_metrics_daily,webmaster_url_requests,webmaster_sitemaps,
webmaster_verifications,webmaster_sites,webmaster_sessions,webmaster_users,
index_outbox,crawl_history,crawl_queue,document_versions,urls,domains,audit_log,system_settings
RESTART IDENTITY CASCADE;
INSERT INTO system_settings(key,value,updated_at) VALUES('resource_pressure','{"state":"NORMAL"}'::jsonb,now())`)
	if err!=nil{t.Fatalf("reset fixture: %v",err)}
	return pool
}

func createVerifiedSite(t *testing.T,repo *Repository,pool *pgxpool.Pool,email,host string)(User,Site){
	t.Helper();ctx:=context.Background()
	user,err:=repo.CreateUser(ctx,email,"test-hash");if err!=nil{t.Fatal(err)}
	site,err:=repo.AddSite(ctx,user.ID,SiteOrigin{Origin:"https://"+host,Host:host});if err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`UPDATE webmaster_sites SET status='VERIFIED',verified_at=now(),verification_method='DNS_TXT' WHERE site_id=$1`,site.ID);err!=nil{t.Fatal(err)}
	site.Status="VERIFIED"
	return user,site
}

func TestWebmasterEmailUniquenessIsCaseInsensitive(t *testing.T){
	pool:=webmasterIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	if _,err:=repo.CreateUser(ctx,"Owner@Example.com","hash");err!=nil{t.Fatal(err)}
	if _,err:=repo.CreateUser(ctx,"owner@example.com","hash");!errors.Is(err,ErrConflict){t.Fatalf("err=%v",err)}
}

func TestWebmasterSessionsAreCappedByDatabase(t *testing.T){
	pool:=webmasterIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	user,err:=repo.CreateUser(ctx,"owner@example.com","hash");if err!=nil{t.Fatal(err)}
	for i:=0;i<15;i++{
		_,hash,err:=wmauth.NewOpaqueToken();if err!=nil{t.Fatal(err)}
		if err:=repo.CreateSession(ctx,user.ID,hash,time.Now().Add(24*time.Hour));err!=nil{t.Fatal(err)}
		time.Sleep(time.Millisecond)
	}
	var count int
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM webmaster_sessions WHERE user_id=$1`,user.ID).Scan(&count);err!=nil{t.Fatal(err)}
	if count!=10{t.Fatalf("session count=%d",count)}
}

func TestOwnedSiteEnforcesTenantIsolation(t *testing.T){
	pool:=webmasterIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	owner,site:=createVerifiedSite(t,repo,pool,"a@example.com","a.test")
	other,err:=repo.CreateUser(ctx,"b@example.com","hash");if err!=nil{t.Fatal(err)}
	if _,err:=repo.OwnedSite(ctx,owner.ID,site.ID,true);err!=nil{t.Fatal(err)}
	if _,err:=repo.OwnedSite(ctx,other.ID,site.ID,false);!errors.Is(err,ErrNotFound){t.Fatalf("cross-tenant err=%v",err)}
}

func TestQueueURLRequestUsesCrawlerAndVersionedDeleteOutbox(t *testing.T){
	pool:=webmasterIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	user,site:=createVerifiedSite(t,repo,pool,"owner@example.com","example.test")
	requestID,err:=repo.QueueURLRequest(ctx,user.ID,site.ID,"https://example.test/page","SUBMIT");if err!=nil{t.Fatal(err)}
	if requestID<=0{t.Fatal("missing request id")}
	var urlID,version,queueCount int64
	if err:=pool.QueryRow(ctx,`SELECT url_id,version FROM urls WHERE normalized_url='https://example.test/page'`).Scan(&urlID,&version);err!=nil{t.Fatal(err)}
	if err:=pool.QueryRow(ctx,`SELECT count(*) FROM crawl_queue WHERE url_id=$1 AND generation=$2 AND status='READY'`,urlID,version).Scan(&queueCount);err!=nil{t.Fatal(err)}
	if queueCount!=1{t.Fatalf("crawl queue count=%d",queueCount)}
	if _,err:=pool.Exec(ctx,`UPDATE webmaster_url_requests SET status='DONE' WHERE request_id=$1`,requestID);err!=nil{t.Fatal(err)}
	if _,err:=repo.QueueURLRequest(ctx,user.ID,site.ID,"https://example.test/page","DELETE");err!=nil{t.Fatal(err)}
	var operation,status string
	var deleteVersion int64
	if err:=pool.QueryRow(ctx,`SELECT operation,status,entity_version FROM index_outbox WHERE entity_type='WEB_DOCUMENT' AND entity_id=$1 ORDER BY id DESC LIMIT 1`,urlID).Scan(&operation,&status,&deleteVersion);err!=nil{t.Fatal(err)}
	if operation!="DELETE" || status!="READY" || deleteVersion<=version{t.Fatalf("operation=%s status=%s version=%d",operation,status,deleteVersion)}
}

func TestWebmasterMetricsAggregateVerifiedHosts(t *testing.T){
	pool:=webmasterIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	user,site:=createVerifiedSite(t,repo,pool,"owner@example.com","example.test")
	if _,err:=pool.Exec(ctx,`INSERT INTO urls(domain_id,normalized_url,index_status) VALUES($1,'https://example.test/page','INDEXED')`,site.DomainID);err!=nil{t.Fatal(err)}
	if err:=repo.RecordSearchImpressions(ctx,[]string{"example.test","example.test","other.test"});err!=nil{t.Fatal(err)}
	if err:=repo.RecordClick(ctx,"https://example.test/page");err!=nil{t.Fatal(err)}
	if err:=repo.RecordClick(ctx,"https://example.test/not-indexed");err!=nil{t.Fatal(err)}
	if err:=repo.RecordAnswerCitations(ctx,[]string{"example.test"});err!=nil{t.Fatal(err)}
	metrics,err:=repo.Metrics(ctx,user.ID,site.ID,time.Now().Add(-24*time.Hour),time.Now().Add(24*time.Hour));if err!=nil{t.Fatal(err)}
	if metrics.Impressions!=2 || metrics.Clicks!=1 || metrics.AnswerCitations!=1{t.Fatalf("metrics=%+v",metrics)}
	if fmt.Sprintf("%.2f",metrics.CTR)!="0.50"{t.Fatalf("ctr=%f",metrics.CTR)}
}

func TestSitemapLeaseRespectsPolicyAndOwnership(t *testing.T){
	pool:=webmasterIntegrationDB(t);repo:=NewRepository(pool);ctx:=context.Background()
	user,site:=createVerifiedSite(t,repo,pool,"owner@example.com","example.test")
	id,err:=repo.QueueSitemapSubmission(ctx,user.ID,site.ID,"https://example.test/sitemap.xml");if err!=nil{t.Fatal(err)}
	tasks,err:=repo.LeaseSitemaps(ctx,"wm-test",10,30);if err!=nil{t.Fatal(err)}
	if len(tasks)!=1 || tasks[0].ID!=id || tasks[0].WorkerID!="wm-test"{t.Fatalf("tasks=%+v",tasks)}
	if err:=repo.RetrySitemap(ctx,tasks[0],errors.New("temporary"),time.Millisecond);err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`UPDATE webmaster_sitemaps SET available_at=now()-interval '1 second' WHERE sitemap_id=$1`,id);err!=nil{t.Fatal(err)}
	if _,err:=pool.Exec(ctx,`UPDATE domains SET policy='BLOCK' WHERE domain_id=$1`,site.DomainID);err!=nil{t.Fatal(err)}
	tasks,err=repo.LeaseSitemaps(ctx,"wm-test-2",10,30);if err!=nil{t.Fatal(err)}
	if len(tasks)!=0{t.Fatalf("blocked domain leased: %+v",tasks)}
}
