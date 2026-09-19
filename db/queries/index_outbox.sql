-- name: EnqueueIndexEvent :one
INSERT INTO index_outbox (
    entity_type,
    entity_id,
    entity_version,
    operation,
    available_at
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (entity_type, entity_id, entity_version)
DO UPDATE SET
    available_at = LEAST(index_outbox.available_at, EXCLUDED.available_at),
    updated_at = now()
RETURNING *;

-- name: SupersedeStaleIndexEvents :execrows
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
  );

-- name: LeaseIndexEvents :many
WITH picked AS (
    SELECT candidate.id
    FROM index_outbox AS candidate
    WHERE candidate.status IN ('READY','RETRY')
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
    LIMIT sqlc.arg(batch_size)
)
UPDATE index_outbox AS o
SET status = 'LEASED',
    lease_until = now() + make_interval(secs => sqlc.arg(lease_seconds)::int),
    worker_id = sqlc.arg(worker_id),
    attempts = o.attempts + 1,
    updated_at = now()
FROM picked
WHERE o.id = picked.id
RETURNING o.*;

-- name: RequeueExpiredIndexLeases :execrows
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
WHERE status = 'LEASED'
  AND lease_until < now();

-- name: MarkIndexEventProcessed :one
UPDATE index_outbox
SET status = 'PROCESSED',
    processed_at = now(),
    lease_until = NULL,
    worker_id = NULL,
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(event_id)
  AND status = 'LEASED'
  AND worker_id = sqlc.arg(worker_id)
RETURNING *;

-- name: RetryIndexEvent :one
UPDATE index_outbox
SET status = CASE WHEN attempts >= max_attempts THEN 'DEAD' ELSE 'RETRY' END,
    available_at = CASE
        WHEN attempts >= max_attempts THEN available_at
        ELSE now() + make_interval(secs => sqlc.arg(retry_after_seconds)::int)
    END,
    lease_until = NULL,
    worker_id = NULL,
    last_error = sqlc.arg(last_error),
    updated_at = now()
WHERE id = sqlc.arg(event_id)
  AND status = 'LEASED'
  AND worker_id = sqlc.arg(worker_id)
RETURNING *;

-- name: GetIndexEvent :one
SELECT * FROM index_outbox WHERE id = $1;

-- name: DeleteProcessedIndexEventsBefore :execrows
DELETE FROM index_outbox
WHERE status IN ('PROCESSED','SUPERSEDED')
  AND processed_at < $1;
