-- name: EnqueueCrawlTask :one
INSERT INTO crawl_queue (
    url_id,
    domain_id,
    generation,
    priority,
    available_at
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (url_id, generation)
WHERE status IN ('READY','LEASED','RETRY')
DO UPDATE SET
    priority = GREATEST(crawl_queue.priority, EXCLUDED.priority),
    available_at = LEAST(crawl_queue.available_at, EXCLUDED.available_at),
    updated_at = now()
RETURNING *;

-- name: LeaseCrawlTasks :many
WITH picked AS (
    SELECT id
    FROM crawl_queue
    WHERE status IN ('READY','RETRY')
      AND available_at <= now()
      AND attempts < max_attempts
    ORDER BY priority DESC, available_at ASC, id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT sqlc.arg(batch_size)
)
UPDATE crawl_queue AS q
SET status = 'LEASED',
    lease_until = now() + make_interval(secs => sqlc.arg(lease_seconds)::int),
    worker_id = sqlc.arg(worker_id),
    attempts = q.attempts + 1,
    updated_at = now()
FROM picked
WHERE q.id = picked.id
RETURNING q.*;

-- name: RequeueExpiredCrawlLeases :execrows
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
WHERE status = 'LEASED'
  AND lease_until < now();

-- name: CompleteCrawlTask :one
UPDATE crawl_queue
SET status = 'DONE',
    lease_until = NULL,
    worker_id = NULL,
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(task_id)
  AND status = 'LEASED'
  AND worker_id = sqlc.arg(worker_id)
RETURNING *;

-- name: RetryCrawlTask :one
UPDATE crawl_queue
SET status = CASE WHEN attempts >= max_attempts THEN 'DEAD' ELSE 'RETRY' END,
    available_at = CASE
        WHEN attempts >= max_attempts THEN available_at
        ELSE now() + make_interval(secs => sqlc.arg(retry_after_seconds)::int)
    END,
    lease_until = NULL,
    worker_id = NULL,
    last_error = sqlc.arg(last_error),
    updated_at = now()
WHERE id = sqlc.arg(task_id)
  AND status = 'LEASED'
  AND worker_id = sqlc.arg(worker_id)
RETURNING *;

-- name: GetCrawlTask :one
SELECT * FROM crawl_queue WHERE id = $1;

-- name: DeleteTerminalCrawlTasksBefore :execrows
DELETE FROM crawl_queue
WHERE status IN ('DONE','DEAD')
  AND updated_at < $1;
