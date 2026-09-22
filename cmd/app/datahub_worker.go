package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/datahub"
)

type dataHubPressureGate struct{pool *pgxpool.Pool}
func (g dataHubPressureGate) Allow(ctx context.Context)(bool,error){
	var ok bool
	err:=g.pool.QueryRow(ctx,`SELECT EXISTS(
 SELECT 1 FROM system_settings
 WHERE key='resource_pressure'
   AND updated_at>now()-interval '60 seconds'
   AND COALESCE(value->>'state','CRITICAL')<>'CRITICAL'
)`).Scan(&ok)
	return ok,err
}

func runDataHubWorker(pool *pgxpool.Pool)error{
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM)
	defer stop()
	worker:=datahub.Worker{
		Repo:datahub.NewRepository(pool),
		Gate:dataHubPressureGate{pool:pool},
		BatchSize:250,
		TrendLimit:200,
		PollInterval:2*time.Minute,
		ErrorBackoff:10*time.Second,
	}
	return worker.Run(ctx)
}
