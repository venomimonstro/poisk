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
	if dsn == "" { t.Fatal("TEST_DATABASE_URL is required for integration tests") }
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil { t.Fatalf("create pool: %v", err) }
	if err := pool.Ping(context.Background()); err != nil { pool.Close(); t.Fatalf("ping database: %v", err) }
	t.Cleanup(pool.Close)
	_, err = pool.Exec(context.Background(), `TRUNCATE TABLE index_outbox RESTART IDENTITY;
INSERT INTO system_settings(key,value,updated_at) VALUES('resource_pressure','{"state":"NORMAL"}'::jsonb,now())
ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`)
	if err != nil { t.Fatalf("reset outbox: %v", err) }
	return pool
}

func TestEnqueueIndexEventIsIdempotent(t *testing.T) {
	pool := integrationDB(t); repo := NewRepository(pool); ctx := context.Background()
	firstID, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 42, 3, "UPSERT", time.Now().Add(time.Minute)); if err != nil { t.Fatalf("first enqueue: %v", err) }
	secondID, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 42, 3, "UPSERT", time.Now()); if err != nil { t.Fatalf("second enqueue: %v", err) }
	if firstID != secondID { t.Fatalf("expected same outbox event id, got %d and %d", firstID, secondID) }
	var count int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM index_outbox WHERE entity_type='WEB_DOCUMENT' AND entity_id=42 AND entity_version=3`).Scan(&count); err != nil { t.Fatalf("count outbox events: %v", err) }
	if count != 1 { t.Fatalf("expected one event, got %d", count) }
}

func TestSameVersionCannotChangeOperation(t *testing.T) {
	pool := integrationDB(t); repo := NewRepository(pool); ctx := context.Background()
	if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 7, 4, "UPSERT", time.Now()); err != nil { t.Fatalf("enqueue original event: %v", err) }
	if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 7, 4, "DELETE", time.Now()); !errors.Is(err, ErrVersionConflict) { t.Fatalf("expected version conflict, got %v", err) }
}

func TestConcurrentIndexLeaseNeverDuplicatesEvent(t *testing.T) {
	pool := integrationDB(t); repo := NewRepository(pool); ctx := context.Background()
	for i := 1; i <= 20; i++ { if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", int64(i), 1, "UPSERT", time.Now()); err != nil { t.Fatalf("enqueue event %d: %v", i, err) } }
	var wg sync.WaitGroup; ids := make(chan int64, 20); errs := make(chan error, 4)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) { defer wg.Done(); events, err := repo.Lease(ctx, fmt.Sprintf("indexer-%d", worker), 5, 30); if err != nil { errs <- err; return }; for _, event := range events { ids <- event.ID } }(worker)
	}
	wg.Wait(); close(ids); close(errs)
	for err := range errs { if err != nil { t.Fatalf("lease error: %v", err) } }
	seen := map[int64]struct{}{}
	for id := range ids { if _, exists := seen[id]; exists { t.Fatalf("event %d leased twice", id) }; seen[id] = struct{}{} }
	if len(seen) != 20 { t.Fatalf("expected 20 unique events, got %d", len(seen)) }
}

func TestNewerVersionSupersedesOlderPendingEvent(t *testing.T) {
	pool := integrationDB(t); repo := NewRepository(pool); ctx := context.Background()
	oldID, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 99, 1, "UPSERT", time.Now()); if err != nil { t.Fatalf("enqueue old version: %v", err) }
	newID, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 99, 2, "DELETE", time.Now()); if err != nil { t.Fatalf("enqueue new version: %v", err) }
	events, err := repo.Lease(ctx, "indexer", 10, 30); if err != nil { t.Fatalf("lease: %v", err) }
	if len(events) != 1 || events[0].ID != newID || events[0].EntityVersion != 2 { t.Fatalf("expected only newest version leased, got %+v", events) }
	var oldStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM index_outbox WHERE id=$1`, oldID).Scan(&oldStatus); err != nil { t.Fatalf("read old event: %v", err) }
	if oldStatus != "SUPERSEDED" { t.Fatalf("expected old event SUPERSEDED, got %s", oldStatus) }
}

func TestNewVersionWaitsForAlreadyLeasedOldVersion(t *testing.T) {
	pool := integrationDB(t); repo := NewRepository(pool); ctx := context.Background()
	if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 55, 1, "UPSERT", time.Now()); err != nil { t.Fatalf("enqueue v1: %v", err) }
	first, err := repo.Lease(ctx, "worker-a", 1, 30); if err != nil || len(first) != 1 { t.Fatalf("lease v1: len=%d err=%v", len(first), err) }
	if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 55, 2, "UPSERT", time.Now()); err != nil { t.Fatalf("enqueue v2: %v", err) }
	second, err := repo.Lease(ctx, "worker-b", 1, 30); if err != nil { t.Fatalf("concurrent lease: %v", err) }
	if len(second) != 0 { t.Fatalf("new version must wait while old version is leased, got %+v", second) }
	if err := repo.MarkProcessed(ctx, first[0].ID, "worker-a"); err != nil { t.Fatalf("finish v1: %v", err) }
	third, err := repo.Lease(ctx, "worker-b", 1, 30); if err != nil || len(third) != 1 || third[0].EntityVersion != 2 { t.Fatalf("expected v2 after v1 completion, events=%+v err=%v", third, err) }
}

func TestIndexLeaseOwnership(t *testing.T) {
	pool := integrationDB(t); repo := NewRepository(pool); ctx := context.Background()
	if _, err := repo.Enqueue(ctx, "WEB_DOCUMENT", 1, 1, "UPSERT", time.Now()); err != nil { t.Fatalf("enqueue: %v", err) }
	events, err := repo.Lease(ctx, "owner", 1, 30); if err != nil || len(events) != 1 { t.Fatalf("lease: len=%d err=%v", len(events), err) }
	if err := repo.MarkProcessed(ctx, events[0].ID, "intruder"); !errors.Is(err, ErrLeaseOwnership) { t.Fatalf("expected ownership error, got %v", err) }
	if err := repo.MarkProcessed(ctx, events[0].ID, "owner"); err != nil { t.Fatalf("owner mark processed: %v", err) }
	var status string; var processedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, processed_at FROM index_outbox WHERE id=$1`, events[0].ID).Scan(&status, &processedAt); err != nil { t.Fatalf("read processed event: %v", err) }
	if status != "PROCESSED" || processedAt == nil { t.Fatalf("unexpected processed state status=%s processed_at=%v", status, processedAt) }
}

func TestEntityScopedLeaseLeavesOrganizationEventsForFutureConsumer(t *testing.T) {
	pool := integrationDB(t); all := NewRepository(pool); web := NewRepositoryForEntityTypes(pool, "WEB_DOCUMENT"); ctx := context.Background()
	webID, err := all.Enqueue(ctx, "WEB_DOCUMENT", 101, 1, "UPSERT", time.Now()); if err != nil { t.Fatal(err) }
	orgID, err := all.Enqueue(ctx, "ORGANIZATION", 202, 1, "UPSERT", time.Now()); if err != nil { t.Fatal(err) }
	events, err := web.Lease(ctx, "web-indexer", 10, 30); if err != nil { t.Fatal(err) }
	if len(events) != 1 || events[0].ID != webID || events[0].EntityType != "WEB_DOCUMENT" { t.Fatalf("web lease=%+v", events) }
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM index_outbox WHERE id=$1`, orgID).Scan(&status); err != nil { t.Fatal(err) }
	if status != "READY" { t.Fatalf("organization event status=%s", status) }
}
