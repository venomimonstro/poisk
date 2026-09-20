package outbox

import (
	"context"
	"errors"
	"fmt"
)

// LeaseType isolates workers by entity type so a WEB_DOCUMENT indexer can never
// consume future ORGANIZATION or ADDRESS events. Under CRITICAL or stale resource
// pressure no new heavy index leases are granted; already leased work can finish.
func (r *Repository) LeaseType(ctx context.Context, workerID, entityType string, batchSize, leaseSeconds int32) ([]Event, error) {
	if workerID == "" { return nil, errors.New("worker id is required") }
	if entityType == "" { return nil, errors.New("entity type is required") }
	if batchSize <= 0 || leaseSeconds <= 0 { return nil, errors.New("batch size and lease seconds must be positive") }
	if _, err := r.SupersedeStale(ctx); err != nil { return nil, err }

	const q = `
WITH picked AS (
    SELECT candidate.id
    FROM index_outbox AS candidate
    WHERE candidate.entity_type = $1
      AND candidate.status IN ('READY','RETRY')
      AND candidate.available_at <= now()
      AND candidate.attempts < candidate.max_attempts
      AND EXISTS (
          SELECT 1 FROM system_settings s
          WHERE s.key='resource_pressure'
            AND s.updated_at >= now()-interval '60 seconds'
            AND COALESCE(s.value->>'state','CRITICAL') <> 'CRITICAL'
      )
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
    LIMIT $2
)
UPDATE index_outbox AS o
SET status = 'LEASED',
    lease_until = now() + make_interval(secs => $3::int),
    worker_id = $4,
    attempts = o.attempts + 1,
    updated_at = now()
FROM picked
WHERE o.id = picked.id
RETURNING o.id, o.entity_type, o.entity_id, o.entity_version,
          o.operation, o.attempts, o.max_attempts, o.lease_until, o.worker_id`

	rows, err := r.db.Query(ctx, q, entityType, batchSize, leaseSeconds, workerID)
	if err != nil { return nil, fmt.Errorf("lease %s index events: %w", entityType, err) }
	defer rows.Close()

	events := make([]Event, 0, batchSize)
	for rows.Next() {
		var event Event
		if err := rows.Scan(
			&event.ID, &event.EntityType, &event.EntityID, &event.EntityVersion,
			&event.Operation, &event.Attempts, &event.MaxAttempts, &event.LeaseUntil, &event.WorkerID,
		); err != nil { return nil, fmt.Errorf("scan leased index event: %w", err) }
		events = append(events, event)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("iterate leased index events: %w", err) }
	return events, nil
}
