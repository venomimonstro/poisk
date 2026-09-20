package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	addressindex "github.com/venomimonstro/poisk/internal/address/indexer"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
	"github.com/venomimonstro/poisk/internal/platform/config"
)

func runAddressIndexer(cfg config.Config,pool *pgxpool.Pool)error{
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM);defer stop()
	index,err:=indexmanticore.New(indexmanticore.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort)});if err!=nil{return err}
	if err:=index.EnsureAddressesSchema(ctx);err!=nil{return fmt.Errorf("ensure address index schema: %w",err)}
	outboxRepo:=outbox.NewRepositoryForEntityTypes(pool,"ADDRESS")
	source:=addressindex.NewSource(pool)
	processor:=&addressindex.Processor{Source:source,Index:index,Ack:outboxRepo,RetryBase:time.Second,RetryMax:time.Minute}
	runner:=addressindex.Runner{Leases:outboxRepo,Processor:processor,WorkerID:fmt.Sprintf("address-indexer-%d",os.Getpid()),BatchSize:64,LeaseSeconds:30,PollInterval:500*time.Millisecond}
	return runner.Run(ctx)
}
