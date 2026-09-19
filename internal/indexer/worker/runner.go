package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/venomimonstro/poisk/internal/indexer/outbox"
)

type LeaseRepository interface {
	LeaseType(ctx context.Context, workerID, entityType string, batchSize, leaseSeconds int32) ([]outbox.Event, error)
	RequeueExpired(ctx context.Context) (int64, error)
}

type Runner struct {
	Leases        LeaseRepository
	Processor     *Processor
	WorkerID      string
	BatchSize     int32
	LeaseSeconds  int32
	PollInterval  time.Duration
}

func (r Runner) Run(ctx context.Context) error {
	if r.Leases == nil || r.Processor == nil { return errors.New("indexer runner dependencies are not initialized") }
	if r.WorkerID == "" { return errors.New("indexer worker id is required") }
	if r.BatchSize <= 0 { r.BatchSize = 32 }
	if r.LeaseSeconds <= 0 { r.LeaseSeconds = 30 }
	if r.PollInterval <= 0 { r.PollInterval = 500 * time.Millisecond }

	ticker := time.NewTicker(r.PollInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil { return err }
		if _, err := r.Leases.RequeueExpired(ctx); err != nil {
			slog.Warn("requeue expired index leases failed", "error", err)
		}
		events, err := r.Leases.LeaseType(ctx, r.WorkerID, WebDocumentEntity, r.BatchSize, r.LeaseSeconds)
		if err != nil {
			slog.Error("lease index events failed", "error", err)
		} else {
			for _, event := range events {
				if err := r.Processor.Process(ctx, event); err != nil {
					slog.Warn("index event failed", "event_id", event.ID, "entity_id", event.EntityID, "version", event.EntityVersion, "error", err)
				}
			}
			if len(events) > 0 { continue }
		}

		select {
		case <-ctx.Done(): return ctx.Err()
		case <-ticker.C:
		}
	}
}
