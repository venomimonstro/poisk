//go:build integration

package outbox

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
	_, err = pool.Exec(context.Background(), `TRUNCATE TABLE index_outbox RESTART IDENTITY`)
	if err != nil {
		t.Fatalf("reset outbox: %v", err)
	}
	return pool
}

func TestEnqueueIndexEventIsIdempotent(t *testing.T) {
	pool := integrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	firstID, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 42, 3, "UPSERT", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	secondID, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 42, 3, "UPSERT", time.Now())
	if err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	if firstID != secondID {
		t.Fatalf("expected same outbox event id, got %d and %d", firstID, secondID)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM index_outbox WHERE entity_type='WEB_DOCUMENT' AND entity_id=42 AND entity_version=3 AND operation='UPSERT'`,
	).Scan(&count); err != nil {
		t.Fatalf("count outbox events: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one event, got %d", count)
	}
}

func TestConcurrentIndexLeaseNeverDuplicatesEvent(t *testing.T) {
	pool := integrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	for i := 1; i <= 20; i++ {
		if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", int64(i), 1, "UPSERT", time.Now()); err != nil {
			t.Fatalf("enqueue event %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	ids := make(chan int64, 20)
	errs := make(chan error, 4)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			events, err := repo.Lease(ctx, fmt.Sprintf("indexer-%d", worker), 5, 30)
			if err != nil {
				errs <- err
				return
			}
			for _, event := range events {
				ids <- event.ID
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
			t.Fatalf("event %d leased twice", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != 20 {
		t.Fatalf("expected 20 unique events, got %d", len(seen))
	}
}

func TestIndexLeaseOwnership(t *testing.T) {
	pool := integrationDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 1, 1, "UPSERT", time.Now()); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	events, err := repo.Lease(ctx, "owner", 1, 30)
	if err != nil || len(events) != 1 {
		t.Fatalf("lease: len=%d err=%v", len(events), err)
	}
	if err := repo.MarkProcessed(ctx, events[0].ID, "intruder"); !errors.Is(err, ErrLeaseOwnership) {
		t.Fatalf("expected ownership error, got %v", err)
	}
	if err := repo.MarkProcessed(ctx, events[0].ID, "owner"); err != nil {
		t.Fatalf("owner mark processed: %v", err)
	}

	var status string
	var processedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, processed_at FROM index_outbox WHERE id=$1`, events[0].ID).Scan(&status, &processedAt); err != nil {
		t.Fatalf("read processed event: %v", err)
	}
	if status != "PROCESSED" || processedAt == nil {
		t.Fatalf("unexpected processed state status=%s processed_at=%v", status, processedAt)
	}
}
