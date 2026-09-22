//go:build integration

package scheduler

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBlockedAndPausedDomainsAreNeverEligible(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" { t.Fatal("TEST_DATABASE_URL is required") }
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatalf("create pool: %v", err) }
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE query_gap_feedback_events,query_gap_domain_feedback,query_gap_domain_observations,query_gaps,query_signal_buckets,index_outbox,crawl_history,crawl_queue,document_versions,urls,domains RESTART IDENTITY CASCADE`); err != nil { t.Fatalf("reset: %v", err) }
	if _, err := pool.Exec(ctx, `
INSERT INTO domains(host,status,policy,crawl_budget,max_urls,demand_score,quality_score) VALUES
('allowed.test','ACTIVE','ALLOW',100,100,10,90),
('limited.test','ACTIVE','LIMITED',50,50,5,80),
('blocked.test','ACTIVE','BLOCK',100,100,100,100),
('paused.test','PAUSED','ALLOW',100,100,100,100),
('zero.test','ACTIVE','ALLOW',0,100,100,100)`); err != nil { t.Fatalf("insert fixtures: %v", err) }

	repo := NewRepository(pool)
	got, err := repo.EligibleDomains(ctx, 20)
	if err != nil { t.Fatalf("EligibleDomains(): %v", err) }
	if len(got) != 2 { t.Fatalf("expected only allowed + limited domains, got %+v", got) }
	if got[0].Host != "allowed.test" || got[1].Host != "limited.test" { t.Fatalf("unexpected order/eligibility: %+v", got) }
	if got[0].RequestsPerSecond <= 0 || got[0].MaxConcurrency <= 0 || got[0].MaxBytesPerDay <= 0 { t.Fatalf("budget fields missing: %+v", got[0]) }
}

func TestActiveGapFeedbackTemporarilyAffectsDomainOrdering(t *testing.T){
	dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};ctx:=context.Background();pool,err:=pgxpool.New(ctx,dsn);if err!=nil{t.Fatal(err)};defer pool.Close()
	_,err=pool.Exec(ctx,`TRUNCATE TABLE query_gap_feedback_events,query_gap_domain_feedback,query_gap_domain_observations,query_gaps,query_signal_buckets,index_outbox,crawl_history,crawl_queue,document_versions,urls,domains RESTART IDENTITY CASCADE`);if err!=nil{t.Fatal(err)}
	var a,b int64
	if err=pool.QueryRow(ctx,`INSERT INTO domains(host,demand_score,quality_score,next_crawl_at) VALUES('a.test',20,80,now()-interval '1 minute') RETURNING domain_id`).Scan(&a);err!=nil{t.Fatal(err)}
	if err=pool.QueryRow(ctx,`INSERT INTO domains(host,demand_score,quality_score,next_crawl_at) VALUES('b.test',30,80,now()-interval '1 minute') RETURNING domain_id`).Scan(&b);err!=nil{t.Fatal(err)}
	var gapID int64;hash:=make([]byte,32);hash[0]=1
	if err=pool.QueryRow(ctx,`INSERT INTO query_gaps(query_hash,state,demand_score,coverage_score,quality_score,freshness_score,spam_score,gap_score,independent_buckets) VALUES($1,'OPEN',80,20,20,30,0,70,4) RETURNING gap_id`,hash).Scan(&gapID);err!=nil{t.Fatal(err)}
	if _,err=pool.Exec(ctx,`INSERT INTO query_gap_domain_feedback(gap_id,domain_id,boost,expires_at,reason) VALUES($1,$2,20,now()+interval '1 hour','LOW_COVERAGE')`,gapID,a);err!=nil{t.Fatal(err)}
	repo:=NewRepository(pool);got,err:=repo.EligibleDomains(ctx,10);if err!=nil{t.Fatal(err)};if len(got)<2||got[0].DomainID!=a{t.Fatalf("active feedback did not reorder: %+v",got)}
	if _,err=pool.Exec(ctx,`UPDATE query_gap_domain_feedback SET expires_at=now()-interval '1 second' WHERE gap_id=$1 AND domain_id=$2`,gapID,a);err!=nil{t.Fatal(err)}
	got,err=repo.EligibleDomains(ctx,10);if err!=nil{t.Fatal(err)};if len(got)<2||got[0].DomainID!=b{t.Fatalf("expired feedback still affected order: %+v",got)}
}
