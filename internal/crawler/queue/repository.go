package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLeaseOwnership = errors.New("crawl task lease is not owned by worker")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type Task struct {
	ID          int64
	URLID       int64
	DomainID    int64
	Generation  int64
	Priority    float64
	Attempts    int32
	MaxAttempts int32
	LeaseUntil  time.Time
	WorkerID    string
}

func (r *Repository) Enqueue(ctx context.Context, urlID, domainID, generation int64, priority float64, availableAt time.Time) (int64, error) {
	const q = `
INSERT INTO crawl_queue (url_id, domain_id, generation, priority, available_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (url_id, generation)
WHERE status IN ('READY','LEASED','RETRY')
DO UPDATE SET
    priority = GREATEST(crawl_queue.priority, EXCLUDED.priority),
    available_at = LEAST(crawl_queue.available_at, EXCLUDED.available_at),
    updated_at = now()
RETURNING id`

	var id int64
	if err := r.db.QueryRow(ctx, q, urlID, domainID, generation, priority, availableAt).Scan(&id); err != nil {
		return 0, fmt.Errorf("enqueue crawl task: %w", err)
	}
	return id, nil
}

func (r *Repository) Lease(ctx context.Context, workerID string, batchSize, leaseSeconds int32) ([]Task, error) {
	if workerID == "" {
		return nil, errors.New("worker id is required")
	}
	if batchSize <= 0 || leaseSeconds <= 0 {
		return nil, errors.New("batch size and lease seconds must be positive")
	}

	const q = `
WITH picked AS (
    SELECT id
    FROM crawl_queue
    WHERE status IN ('READY','RETRY')
      AND available_at <= now()
      AND attempts < max_attempts
      AND COALESCE((SELECT value->>'state' FROM system_settings WHERE key='resource_pressure'),'NORMAL') <> 'CRITICAL'
    ORDER BY priority DESC, available_at ASC, id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
UPDATE crawl_queue AS q
SET status = 'LEASED',
    lease_until = now() + make_interval(secs => $2::int),
    worker_id = $3,
    attempts = q.attempts + 1,
    updated_at = now()
FROM picked
WHERE q.id = picked.id
RETURNING q.id, q.url_id, q.domain_id, q.generation, q.priority,
          q.attempts, q.max_attempts, q.lease_until, q.worker_id`

	rows, err := r.db.Query(ctx, q, batchSize, leaseSeconds, workerID)
	if err != nil {
		return nil, fmt.Errorf("lease crawl tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0, batchSize)
	for rows.Next() {
		var task Task
		if err := rows.Scan(
			&task.ID,
			&task.URLID,
			&task.DomainID,
			&task.Generation,
			&task.Priority,
			&task.Attempts,
			&task.MaxAttempts,
			&task.LeaseUntil,
			&task.WorkerID,
		); err != nil {
			return nil, fmt.Errorf("scan leased crawl task: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leased crawl tasks: %w", err)
	}
	return tasks, nil
}

func (r *Repository) RequeueExpired(ctx context.Context) (int64, error) {
	const q = `
UPDATE crawl_queue
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
		return 0, fmt.Errorf("requeue expired crawl leases: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) Complete(ctx context.Context, taskID int64, workerID string) error {
	const q = `
UPDATE crawl_queue
SET status = 'DONE', lease_until = NULL, worker_id = NULL,
    last_error = NULL, updated_at = now()
WHERE id = $1 AND status = 'LEASED' AND worker_id = $2`

	tag, err := r.db.Exec(ctx, q, taskID, workerID)
	if err != nil {
		return fmt.Errorf("complete crawl task: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseOwnership
	}
	return nil
}

func (r *Repository) Retry(ctx context.Context, taskID int64, workerID, lastError string, retryAfter time.Duration) error {
	if retryAfter < 0 {
		return errors.New("retry delay cannot be negative")
	}
	seconds := int32(retryAfter / time.Second)

	const q = `
UPDATE crawl_queue
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

	tag, err := r.db.Exec(ctx, q, taskID, workerID, seconds, lastError)
	if err != nil {
		return fmt.Errorf("retry crawl task: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseOwnership
	}
	return nil
}

func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
