package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/organizations"
)

func runOrganizationsWorker(pool *pgxpool.Pool)error{
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM)
	defer stop()
	workerID:=fmt.Sprintf("organizations-%d",os.Getpid())
	runner:=organizations.ApplyRunner{
		Store:organizations.NewRepository(pool),
		WorkerID:workerID,
		LeaseDuration:60*time.Second,
		PollInterval:time.Second,
	}
	slog.Info("organization import apply worker started","worker_id",workerID)
	return runner.Run(ctx)
}
