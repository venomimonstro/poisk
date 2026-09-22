package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
	platformresources "github.com/venomimonstro/poisk/internal/platform/resources"
	"github.com/venomimonstro/poisk/internal/reviews"
)

func runResourceMonitor(pool *pgxpool.Pool) error {
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM)
	defer stop()
	monitor:=platformresources.Monitor{
		DB:pool,
		Path:getenvString("RESOURCE_DISK_PATH","/"),
		HighDiskPct:getenvFloat("RESOURCE_HIGH_DISK_PCT",85),
		CriticalDiskPct:getenvFloat("RESOURCE_CRITICAL_DISK_PCT",93),
		HighMemoryPct:getenvFloat("RESOURCE_HIGH_MEMORY_PCT",85),
		CriticalMemoryPct:getenvFloat("RESOURCE_CRITICAL_MEMORY_PCT",95),
	}
	go runReviewMaintenance(ctx,reviews.Repository{DB:pool})
	go runMailMaintenance(ctx,mailcore.Repository{DB:pool},getenvString("MAIL_BLOB_DIR","/mail-blobs"))
	return monitor.Run(ctx,5*time.Second)
}

func runReviewMaintenance(ctx context.Context,repo reviews.Repository){
	prune:=func(){if err:=repo.PruneActionBuckets(ctx,time.Now().UTC());err!=nil&&ctx.Err()==nil{slog.Warn("review maintenance prune failed","error",err)}}
	prune()
	ticker:=time.NewTicker(6*time.Hour);defer ticker.Stop()
	for{select{case<-ctx.Done():return;case<-ticker.C:prune()}}
}

func runMailMaintenance(ctx context.Context,repo mailcore.Repository,root string){
	store:=mailcore.AttachmentStore{Repo:repo,Root:root}
	prune:=func(){now:=time.Now().UTC();if err:=repo.PruneMaintenance(ctx,now);err!=nil&&ctx.Err()==nil{slog.Warn("mail retention prune failed","error",err)};if err:=store.RunBlobGC(ctx,now,100);err!=nil&&ctx.Err()==nil{slog.Warn("mail blob gc failed","error",err)}}
	prune()
	ticker:=time.NewTicker(6*time.Hour);defer ticker.Stop()
	for{select{case<-ctx.Done():return;case<-ticker.C:prune()}}
}

func getenvString(key,fallback string)string{if value:=strings.TrimSpace(os.Getenv(key));value!=""{return value};return fallback}
func getenvFloat(key string,fallback float64)float64{raw:=strings.TrimSpace(os.Getenv(key));if raw==""{return fallback};value,err:=strconv.ParseFloat(raw,64);if err!=nil{return fallback};return value}
