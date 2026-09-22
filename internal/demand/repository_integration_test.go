//go:build integration

package demand

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func demandDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")}
	p,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=p.Ping(context.Background());err!=nil{p.Close();t.Fatal(err)};t.Cleanup(p.Close)
	_,err=p.Exec(context.Background(),`TRUNCATE TABLE query_gap_feedback_events,query_gap_domain_feedback,query_gap_domain_observations,query_gaps,query_signal_buckets,crawl_queue,urls,domains RESTART IDENTITY CASCADE`);if err!=nil{t.Fatal(err)}
	return p
}

func TestBucketCapAndQualifiedFeedback(t *testing.T){
	p:=demandDB(t);ctx:=context.Background();repo:=NewRepository(p)
	var domainID int64
	if err:=p.QueryRow(ctx,`INSERT INTO domains(host,status,policy,next_crawl_at) VALUES('known.test','ACTIVE','ALLOW',now()+interval '2 days') RETURNING domain_id`).Scan(&domainID);err!=nil{t.Fatal(err)}
	base:=time.Date(2026,9,22,6,0,0,0,time.UTC)
	poor:=Snapshot{Normalized:"редкий запрос",Total:0,AverageQuality:0,AverageFreshness:0,AverageSpam:100,Hosts:[]string{"known.test","unknown.test"}}
	for i:=0;i<100;i++{if _,err:=repo.Record(ctx,poor,base.Add(time.Duration(i%9)*time.Second));err!=nil{t.Fatal(err)}}
	var hits,resultSum,qualitySum,spamSum int
	if err:=p.QueryRow(ctx,`SELECT hits,result_count_sum,quality_sum,spam_sum FROM query_signal_buckets`).Scan(&hits,&resultSum,&qualitySum,&spamSum);err!=nil{t.Fatal(err)}
	if hits!=3||resultSum!=0||qualitySum!=0||spamSum!=300{t.Fatalf("bucket cap failed hits=%d results=%d quality=%d spam=%d",hits,resultSum,qualitySum,spamSum)}
	var observations int
	if err:=p.QueryRow(ctx,`SELECT count(*) FROM query_gap_domain_observations`).Scan(&observations);err!=nil{t.Fatal(err)};if observations!=1{t.Fatalf("observations=%d want=1",observations)}

	for _,offset:=range []time.Duration{10*time.Minute,20*time.Minute}{if _,err:=repo.Record(ctx,poor,base.Add(offset));err!=nil{t.Fatal(err)}}
	var state string;var buckets int
	if err:=p.QueryRow(ctx,`SELECT state,independent_buckets FROM query_gaps`).Scan(&state,&buckets);err!=nil{t.Fatal(err)}
	if state!="OPEN"||buckets<3{t.Fatalf("state=%s buckets=%d",state,buckets)}
	stats,err:=repo.MaterializeFeedback(ctx,20,base.Add(25*time.Minute));if err!=nil{t.Fatal(err)};if stats.Boosted!=1{t.Fatalf("boosted=%d",stats.Boosted)}
	var boost int;var expires,next time.Time
	if err:=p.QueryRow(ctx,`SELECT f.boost,f.expires_at,d.next_crawl_at FROM query_gap_domain_feedback f JOIN domains d ON d.domain_id=f.domain_id WHERE f.domain_id=$1`,domainID).Scan(&boost,&expires,&next);err!=nil{t.Fatal(err)}
	if boost<1||boost>25{t.Fatalf("boost=%d",boost)}
	if !expires.After(base.Add(25*time.Minute))||next.After(base.Add(56*time.Minute)){t.Fatalf("expires=%s next=%s",expires,next)}
}

func TestSingleBucketCannotCreateFeedback(t *testing.T){
	p:=demandDB(t);ctx:=context.Background();repo:=NewRepository(p)
	if _,err:=p.Exec(ctx,`INSERT INTO domains(host,status,policy) VALUES('known.test','ACTIVE','ALLOW')`);err!=nil{t.Fatal(err)}
	now:=time.Date(2026,9,22,7,0,0,0,time.UTC)
	for i:=0;i<100;i++{_,err:=repo.Record(ctx,Snapshot{Normalized:"spam query",Total:0,AverageQuality:0,AverageFreshness:0,AverageSpam:100,Hosts:[]string{"known.test"}},now.Add(time.Duration(i)*time.Second));if err!=nil{t.Fatal(err)}}
	stats,err:=repo.MaterializeFeedback(ctx,20,now.Add(2*time.Minute));if err!=nil{t.Fatal(err)};if stats.Boosted!=0{t.Fatalf("single bucket produced boost=%d",stats.Boosted)}
	var count int;if err:=p.QueryRow(ctx,`SELECT count(*) FROM query_gap_domain_feedback`).Scan(&count);err!=nil{t.Fatal(err)};if count!=0{t.Fatalf("feedback rows=%d",count)}
}
