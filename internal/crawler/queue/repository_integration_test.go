//go:build integration

package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func integrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(pool.Close)
	resetQueueFixture(t, pool)
	return pool
}

func resetQueueFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
TRUNCATE TABLE index_outbox, crawl_history, crawl_queue, document_versions,
               urls, domains, audit_log, system_settings
RESTART IDENTITY CASCADE;
INSERT INTO system_settings(key,value,updated_at)
VALUES('resource_pressure','{"state":"NORMAL"}'::jsonb,now())`)
	if err != nil {
		t.Fatalf("reset fixture: %v", err)
	}
}

func createDomainAndURL(t *testing.T, pool *pgxpool.Pool, suffix int) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var domainID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO domains(host) VALUES ($1) RETURNING domain_id`,
		fmt.Sprintf("example-%d.test", suffix),
	).Scan(&domainID); err != nil {
		t.Fatalf("insert domain: %v", err)
	}

	var urlID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO urls(domain_id, normalized_url) VALUES ($1,$2) RETURNING url_id`,
		domainID,
		fmt.Sprintf("https://example-%d.test/page", suffix),
	).Scan(&urlID); err != nil {
		t.Fatalf("insert url: %v", err)
	}
	return domainID, urlID
}

func TestEnqueueIsIdempotentForActiveGeneration(t *testing.T) {
	pool := integrationDB(t)
	repo := NewRepository(pool)
	domainID, urlID := createDomainAndURL(t, pool, 1)
	ctx := context.Background()

	firstID, err := repo.Enqueue(ctx, urlID, domainID, 1, 1, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	secondID, err := repo.Enqueue(ctx, urlID, domainID, 1, 10, time.Now())
	if err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	if firstID != secondID {
		t.Fatalf("expected same active task id, got %d and %d", firstID, secondID)
	}

	var count int
	var priority float64
	if err := pool.QueryRow(ctx,
		`SELECT count(*), max(priority) FROM crawl_queue WHERE url_id=$1 AND generation=1 AND status IN ('READY','LEASED','RETRY')`,
		urlID,
	).Scan(&count, &priority); err != nil {
		t.Fatalf("read active tasks: %v", err)
	}
	if count != 1 || priority != 10 {
		t.Fatalf("expected one upgraded active task, count=%d priority=%v", count, priority)
	}
}

func TestConcurrentLeaseNeverReturnsSameTask(t *testing.T) {
	pool := integrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		domainID, urlID := createDomainAndURL(t, pool, 100+i)
		if _, err := repo.Enqueue(ctx, urlID, domainID, 1, float64(100-i), time.Now()); err != nil {
			t.Fatalf("enqueue task %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	ids := make(chan int64, 20)
	errs := make(chan error, 4)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			tasks, err := repo.Lease(ctx, fmt.Sprintf("worker-%d", worker), 5, 30)
			if err != nil {
				errs <- err
				return
			}
			for _, task := range tasks {
				ids <- task.ID
			}
		}(worker)
	}
	wg.Wait()
	close(ids)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("lease error: %v", err)
		}
	}
	seen := map[int64]struct{}{}
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("task %d leased twice", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != 20 {
		t.Fatalf("expected 20 uniquely leased tasks, got %d", len(seen))
	}
}

func TestLeaseOwnershipAndExpiry(t *testing.T) {
	pool := integrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	domainID, urlID := createDomainAndURL(t, pool, 2)
	if _, err := repo.Enqueue(ctx, urlID, domainID, 1, 1, time.Now()); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	tasks, err := repo.Lease(ctx, "owner", 1, 30)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("lease: tasks=%d err=%v", len(tasks), err)
	}
	if err := repo.Complete(ctx, tasks[0].ID, "intruder"); !errors.Is(err, ErrLeaseOwnership) {
		t.Fatalf("expected lease ownership error, got %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE crawl_queue SET lease_until=now()-interval '1 second' WHERE id=$1`,
		tasks[0].ID,
	); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	count, err := repo.RequeueExpired(ctx)
	if err != nil || count != 1 {
		t.Fatalf("requeue expired: count=%d err=%v", count, err)
	}

	var status string
	var workerID *string
	if err := pool.QueryRow(ctx, `SELECT status, worker_id FROM crawl_queue WHERE id=$1`, tasks[0].ID).Scan(&status, &workerID); err != nil {
		t.Fatalf("read task: %v", err)
	}
	if status != "RETRY" || workerID != nil {
		t.Fatalf("expected RETRY without owner, status=%s worker=%v", status, workerID)
	}
}
