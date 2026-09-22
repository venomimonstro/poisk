package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/demand"
)

func runDemandWorker(pool *pgxpool.Pool) error {
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM)
	defer stop()
	repo:=demand.NewRepository(pool)
	workerID:=os.Getpid()
	ticker:=time.NewTicker(5*time.Minute)
	defer ticker.Stop()
	run:=func(){
		workCtx,cancel:=context.WithTimeout(ctx,20*time.Second)
		stats,err:=repo.MaterializeFeedback(workCtx,200,time.Now().UTC())
		cancel()
		if err!=nil{slog.Error("demand feedback iteration failed","worker_id",workerID,"error",err);return}
		if stats.Boosted>0||stats.Expired>0{slog.Info("demand feedback iteration","worker_id",workerID,"boosted",stats.Boosted,"expired",stats.Expired)}
	}
	run()
	for{
		select{
		case <-ctx.Done():return ctx.Err()
		case <-ticker.C:run()
		}
	}
}
