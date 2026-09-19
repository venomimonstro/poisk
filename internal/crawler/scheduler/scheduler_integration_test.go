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
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `TRUNCATE TABLE index_outbox, crawl_history, crawl_queue, document_versions, urls, domains RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO domains(host,status,policy,crawl_budget,max_urls,demand_score,quality_score) VALUES
('allowed.test','ACTIVE','ALLOW',100,100,10,90),
('limited.test','ACTIVE','LIMITED',50,50,5,80),
('blocked.test','ACTIVE','BLOCK',100,100,100,100),
('paused.test','PAUSED','ALLOW',100,100,100,100),
('zero.test','ACTIVE','ALLOW',0,100,100,100)`); err != nil {
		t.Fatalf("insert fixtures: %v", err)
	}

	repo := NewRepository(pool)
	got, err := repo.EligibleDomains(ctx, 20)
	if err != nil {
		t.Fatalf("EligibleDomains(): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected only allowed + limited domains, got %+v", got)
	}
	if got[0].Host != "allowed.test" || got[1].Host != "limited.test" {
		t.Fatalf("unexpected order/eligibility: %+v", got)
	}
	if got[0].RequestsPerSecond <= 0 || got[0].MaxConcurrency <= 0 || got[0].MaxBytesPerDay <= 0 {
		t.Fatalf("budget fields missing: %+v", got[0])
	}
}
