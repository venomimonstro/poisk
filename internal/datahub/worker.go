package datahub

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type Worker struct {
	Repo          *Repository
	BatchSize     int
	TrendLimit    int
	PollInterval  time.Duration
	ErrorBackoff  time.Duration
}

func (w Worker) Run(ctx context.Context) error {
	if w.Repo == nil || w.Repo.db == nil {
		return errors.New("data hub repository is not initialized")
	}
	if w.BatchSize <= 0 || w.BatchSize > 1000 { w.BatchSize = 250 }
	if w.TrendLimit <= 0 || w.TrendLimit > 500 { w.TrendLimit = 200 }
	if w.PollInterval <= 0 { w.PollInterval = 2 * time.Second }
	if w.ErrorBackoff <= 0 { w.ErrorBackoff = 5 * time.Second }

	for {
		if err := ctx.Err(); err != nil { return err }
		now := time.Now().UTC()
		progress := false

		for _, job := range []struct {
			name string
			run  func(context.Context, int, time.Time) (BatchStats, error)
		}{
			{"DIRECTORIES", w.Repo.RebuildDirectoryBatch},
			{"ORGANIZATIONS", w.Repo.RebuildOrganizationBatch},
			{"WEBSITES", w.Repo.RebuildWebsiteBatch},
		} {
			stats, err := job.run(ctx, w.BatchSize, now)
			if err != nil {
				slog.Error("data hub materialization batch failed", "job", job.name, "error", err)
				if err := sleepContext(ctx, w.ErrorBackoff); err != nil { return err }
				continue
			}
			if stats.Scanned > 0 || stats.Changed > 0 { progress = true }
			slog.Info("data hub materialization batch", "job", job.name, "scanned", stats.Scanned, "changed", stats.Changed, "published", stats.Published, "suppressed", stats.Suppressed, "completed", stats.Completed)
		}

		if _, err := w.Repo.MaterializeTrends(ctx, now, w.TrendLimit); err != nil {
			slog.Error("data hub trends materialization failed", "error", err)
		}

		wait := w.PollInterval
		if progress { wait = 100 * time.Millisecond }
		if err := sleepContext(ctx, wait); err != nil { return err }
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done(): return ctx.Err()
	case <-t.C: return nil
	}
}
