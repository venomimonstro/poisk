package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
	orgindex "github.com/venomimonstro/poisk/internal/organizations/indexer"
	"github.com/venomimonstro/poisk/internal/platform/config"
)

func runOrganizationIndexer(cfg config.Config, pool *pgxpool.Pool) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	index, err := indexmanticore.New(indexmanticore.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort)})
	if err != nil { return fmt.Errorf("create organization index client: %w", err) }
	if err := index.EnsureOrganizationsSchema(ctx); err != nil { return fmt.Errorf("ensure organizations index schema: %w", err) }

	outboxRepo := outbox.NewRepositoryForEntityTypes(pool, "ORGANIZATION")
	source := orgindex.NewSource(pool)
	processor := &orgindex.Processor{Source: source, Index: index, Ack: outboxRepo, RetryBase: time.Second, RetryMax: time.Minute}
	runner := orgindex.Runner{Leases: outboxRepo, Processor: processor, WorkerID: fmt.Sprintf("organization-indexer-%d", os.Getpid()), BatchSize: 32, LeaseSeconds: 30, PollInterval: 500 * time.Millisecond}
	return runner.Run(ctx)
}
