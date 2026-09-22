package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

func runMailGatewayWorker(pool *pgxpool.Pool) error {
	if pool==nil{return errors.New("mail gateway worker database is not initialized")}
	baseURL:=strings.TrimSpace(os.Getenv("MAIL_MTA_BASE_URL"));secret:=strings.TrimSpace(os.Getenv("MAIL_GATEWAY_SHARED_SECRET"));blobRoot:=strings.TrimSpace(os.Getenv("MAIL_BLOB_DIR"));if blobRoot==""{blobRoot="/mail-blobs"}
	if baseURL==""{return errors.New("MAIL_MTA_BASE_URL is required for mail-gateway-worker")};if len(secret)<32{return errors.New("MAIL_GATEWAY_SHARED_SECRET must be at least 32 bytes")}
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM);defer stop()
	workerID:=fmt.Sprintf("mail-gateway-%d",os.Getpid())
	worker:=mailcore.GatewayWorker{
		Repo:mailcore.Repository{DB:pool},
		MTA:mailcore.MTAClient{BaseURL:baseURL,Secret:[]byte(secret),HTTP:&http.Client{Timeout:20*time.Second}},
		BlobRoot:blobRoot,
		WorkerID:workerID,
		BatchSize:16,
		Lease:45*time.Second,
		Poll:time.Second,
	}
	return worker.Run(ctx)
}
