package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLeaseOwnership = errors.New("index event lease is not owned by worker")
	ErrVersionConflict = errors.New("entity version already has a different index operation")
)

type Repository struct {
	db *pgxpool.Pool
	entityTypes []string
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func NewRepositoryForEntityTypes(db *pgxpool.Pool, entityTypes ...string) *Repository {
	filtered:=make([]string,0,len(entityTypes))
	seen:=make(map[string]struct{},len(entityTypes))
	for _,value:=range entityTypes{
		if value!="WEB_DOCUMENT"&&value!="ORGANIZATION"&&value!="ADDRESS"{continue}
		if _,ok:=seen[value];ok{continue};seen[value]=struct{}{};filtered=append(filtered,value)
	}
	return &Repository{db:db,entityTypes:filtered}
}

type Event struct {
	ID            int64
	EntityType    string
	EntityID      int64
	EntityVersion int64
	Operation     string
	Attempts      int32
	MaxAttempts   int32
	LeaseUntil    time.Time
	WorkerID      string
}

func (r *Repository) Enqueue(ctx context.Context, entityType string, entityID, entityVersion int64, operation string, availableAt time.Time) (int64, error) {
	const q = `
INSERT INTO index_outbox (entity_type, entity_id, entity_version, operation, available_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (entity_type, entity_id, entity_version)
DO UPDATE SET
    available_at = LEAST(index_outbox.available_at, EXCLUDED.available_at),
    updated_at = now()
RETURNING id, operation`

	var id int64
	var storedOperation string
	if err := r.db.QueryRow(ctx, q, entityType, entityID, entityVersion, operation, availableAt).Scan(&id, &storedOperation); err != nil {
		return 0, fmt.Errorf("enqueue index event: %w", err)
	}
	if storedOperation != operation {
		return 0, fmt.Errorf("%w: entity=%s/%d version=%d existing=%s requested=%s", ErrVersionConflict, entityType, entityID, entityVersion, storedOperation, operation)
	}
	return id, nil
}

func (r *Repository) SupersedeStale(ctx context.Context) (int64, error) {
	const q = `
UPDATE index_outbox AS stale
SET status = 'SUPERSEDED',
    processed_at = now(),
    lease_until = NULL,
    worker_id = NULL,
    last_error = 'superseded_by_newer_version',
    updated_at = now()
WHERE stale.status IN ('READY','RETRY')
  AND EXISTS (
      SELECT 1
      FROM index_outbox AS newer
      WHERE newer.entity_type = stale.entity_type
        AND newer.entity_id = stale.entity_id
        AND newer.entity_version > stale.entity_version
  )`

	tag, err := r.db.Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("supersede stale index events: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) Lease(ctx context.Context, workerID string, batchSize, leaseSeconds int32) ([]Event, error) {
	if workerID == "" {
		return nil, errors.New("worker id is required")
	}
	if batchSize <= 0 || leaseSeconds <= 0 {
		return nil, errors.New("batch size and lease seconds must be positive")
	}

	if _, err := r.SupersedeStale(ctx); err != nil {
		return nil, err
	}
	entityTypes:=r.entityTypes
	if len(entityTypes)==0{entityTypes=[]string{"WEB_DOCUMENT","ORGANIZATION","ADDRESS"}}

	const q = `
WITH picked AS (
    SELECT candidate.id
    FROM index_outbox AS candidate
    WHERE candidate.status IN ('READY','RETRY')
      AND candidate.entity_type = ANY($4::text[])
      AND candidate.available_at <= now()
      AND candidate.attempts < candidate.max_attempts
      AND NOT EXISTS (
          SELECT 1
          FROM index_outbox AS newer
          WHERE newer.entity_type = candidate.entity_type
            AND newer.entity_id = candidate.entity_id
            AND newer.entity_version > candidate.entity_version
      )
      AND NOT EXISTS (
          SELECT 1
          FROM index_outbox AS held
          WHERE held.entity_type = candidate.entity_type
            AND held.entity_id = candidate.entity_id
            AND held.status = 'LEASED'
      )
    ORDER BY candidate.available_at ASC, candidate.id ASC
    FOR UPDATE OF candidate SKIP LOCKED
    LIMIT $1
)
UPDATE index_outbox AS o
SET status = 'LEASED',
    lease_until = now() + make_interval(secs => $2::int),
    worker_id = $3,
    attempts = o.attempts + 1,
    updated_at = now()
FROM picked
WHERE o.id = picked.id
RETURNING o.id, o.entity_type, o.entity_id, o.entity_version,
          o.operation, o.attempts, o.max_attempts, o.lease_until, o.worker_id`

	rows, err := r.db.Query(ctx, q, batchSize, leaseSeconds, workerID, entityTypes)
	if err != nil {
		return nil, fmt.Errorf("lease index events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, batchSize)
	for rows.Next() {
		var event Event
		if err := rows.Scan(
			&event.ID,
			&event.EntityType,
			&event.EntityID,
			&event.EntityVersion,
			&event.Operation,
			&event.Attempts,
			&event.MaxAttempts,
			&event.LeaseUntil,
			&event.WorkerID,
		); err != nil {
			return nil, fmt.Errorf("scan leased index event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leased index events: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkProcessed(ctx context.Context, eventID int64, workerID string) error {
	const q = `
UPDATE index_outbox
SET status = 'PROCESSED', processed_at = now(), lease_until = NULL,
    worker_id = NULL, last_error = NULL, updated_at = now()
WHERE id = $1 AND status = 'LEASED' AND worker_id = $2`

	tag, err := r.db.Exec(ctx, q, eventID, workerID)
	if err != nil {
		return fmt.Errorf("mark index event processed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseOwnership
	}
	return nil
}

func (r *Repository) Retry(ctx context.Context, eventID int64, workerID, lastError string, retryAfter time.Duration) error {
	if retryAfter < 0 {
		return errors.New("retry delay cannot be negative")
	}
	seconds := int32(retryAfter / time.Second)

	const q = `
UPDATE index_outbox
SET status = CASE WHEN attempts >= max_attempts THEN 'DEAD' ELSE 'RETRY' END,
    available_at = CASE
        WHEN attempts >= max_attempts THEN available_at
        ELSE now() + make_interval(secs => $3::int)
    END,
    lease_until = NULL,
    worker_id = NULL,
    last_error = $4,
    updated_at = now()
WHERE id = $1 AND status = 'LEASED' AND worker_id = $2`

	tag, err := r.db.Exec(ctx, q, eventID, workerID, seconds, lastError)
	if err != nil {
		return fmt.Errorf("retry index event: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseOwnership
	}
	return nil
}

func (r *Repository) RequeueExpired(ctx context.Context) (int64, error) {
	const q = `
UPDATE index_outbox
SET status = CASE WHEN attempts >= max_attempts THEN 'DEAD' ELSE 'RETRY' END,
    available_at = CASE WHEN attempts >= max_attempts THEN available_at ELSE now() END,
    lease_until = NULL,
    worker_id = NULL,
    last_error = CASE
        WHEN attempts >= max_attempts THEN COALESCE(last_error, 'lease_expired_max_attempts')
        ELSE COALESCE(last_error, 'lease_expired')
    END,
    updated_at = now()
WHERE status = 'LEASED' AND lease_until < now()`

	tag, err := r.db.Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("requeue expired index leases: %w", err)
	}
	return tag.RowsAffected(), nil
}
