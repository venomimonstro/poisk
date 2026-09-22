package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/datahub"
)

func runDataHubWorker(pool *pgxpool.Pool)error{
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM);defer stop();repo:=datahub.NewRepository(pool);workerID:=os.Getpid();ticker:=time.NewTicker(2*time.Minute);defer ticker.Stop()
	run:=func(){
		workCtx,cancel:=context.WithTimeout(ctx,45*time.Second);defer cancel();ok,err:=dataHubResourcesAvailable(workCtx,pool);if err!=nil||!ok{if err!=nil{slog.Warn("datahub resource gate failed","worker_id",workerID,"error",err)};return}
		now:=time.Now().UTC();dirs,err:=repo.RebuildDirectoryBatch(workCtx,250,now);if err!=nil{slog.Error("datahub directory batch failed","worker_id",workerID,"error",err);return};orgs,err:=repo.RebuildOrganizationBatch(workCtx,250,now);if err!=nil{slog.Error("datahub organization batch failed","worker_id",workerID,"error",err);return};sites,err:=repo.RebuildWebsiteBatch(workCtx,250,now);if err!=nil{slog.Error("datahub website batch failed","worker_id",workerID,"error",err);return};trends,err:=repo.MaterializeTrends(workCtx,now,200);if err!=nil{slog.Error("datahub trends failed","worker_id",workerID,"error",err);return}
		slog.Info("datahub batch","worker_id",workerID,"directories_scanned",dirs.Scanned,"organizations_scanned",orgs.Scanned,"websites_scanned",sites.Scanned,"trends",trends)
	}
	run();for{select{case<-ctx.Done():return ctx.Err();case<-ticker.C:run()}}
}

func dataHubResourcesAvailable(ctx context.Context,pool *pgxpool.Pool)(bool,error){
	var ok bool;err:=pool.QueryRow(ctx,`SELECT EXISTS(
 SELECT 1 FROM system_settings
 WHERE key='resource_pressure'
   AND updated_at>now()-interval '60 seconds'
   AND COALESCE(value->>'state','CRITICAL')<>'CRITICAL'
)`).Scan(&ok);return ok,err
}
