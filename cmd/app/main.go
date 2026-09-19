package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
	"github.com/venomimonstro/poisk/internal/indexer/source"
	indexworker "github.com/venomimonstro/poisk/internal/indexer/worker"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/health"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
	"github.com/venomimonstro/poisk/internal/quality"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
	searchhttp "github.com/venomimonstro/poisk/internal/search/httpapi"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil { return err }

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	mode := "api"
	if len(os.Args) > 1 { mode = os.Args[1] }

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil { return fmt.Errorf("create postgres pool: %w", err) }
	defer pool.Close()

	switch mode {
	case "migrate":
		if err := migrate.Up(ctx, pool, cfg.MigrationsDir); err != nil { return err }
		slog.Info("migrations applied")
		return nil
	case "api":
		return runAPI(cfg, pool)
	case "indexer":
		return runIndexer(cfg, pool)
	case "quality":
		return runQuality(cfg)
	default:
		return fmt.Errorf("runtime mode %q is not implemented in current sprint", mode)
	}
}

func runAPI(cfg config.Config, pool *pgxpool.Pool) error {
	checker := health.Checker{DB: pool, ManticoreHost: cfg.ManticoreHost, ManticoreSQLPort: cfg.ManticoreSQLPort}

	searchBackend, err := searchbackend.New(searchbackend.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort)})
	if err != nil { return fmt.Errorf("create search backend: %w", err) }
	searchService := &searchsvc.Service{Backend: searchBackend, Cache: searchsvc.NewCache(512)}
	searchHandler := searchhttp.Handler{SearchService: searchService}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Get("/health/live", checker.Live)
	router.Get("/health/ready", checker.Ready)
	router.Get("/api/search", searchHandler.Search)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("http server started", "addr", cfg.Addr, "env", cfg.Env, "mode", "api")
		serverErr <- server.ListenAndServe()
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		slog.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) { return err }
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func runIndexer(cfg config.Config, pool *pgxpool.Pool) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	index, err := indexmanticore.New(indexmanticore.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort)})
	if err != nil { return fmt.Errorf("create manticore index client: %w", err) }
	if err := index.EnsureSchema(ctx); err != nil { return fmt.Errorf("ensure web index schema: %w", err) }

	outboxRepo := outbox.NewRepository(pool)
	sourceRepo := source.NewRepository(pool)
	processor := &indexworker.Processor{
		Source: sourceRepo,
		Index: index,
		Ack: outboxRepo,
		RetryBase: time.Second,
		RetryMax: time.Minute,
	}
	runner := indexworker.Runner{
		Leases: outboxRepo,
		Processor: processor,
		WorkerID: fmt.Sprintf("indexer-%d", os.Getpid()),
		BatchSize: 32,
		LeaseSeconds: 30,
		PollInterval: 500 * time.Millisecond,
	}
	slog.Info("indexer worker started", "worker_id", runner.WorkerID)
	return runner.Run(ctx)
}

func runQuality(cfg config.Config) error {
	goldenPath := os.Getenv("QUALITY_GOLDEN_PATH")
	if goldenPath == "" { goldenPath = "/app/docs/quality/golden.seed.json" }
	thresholdPath := os.Getenv("QUALITY_THRESHOLDS_PATH")
	if thresholdPath == "" { thresholdPath = "/app/docs/quality/thresholds.json" }

	goldenFile, err := os.Open(goldenPath)
	if err != nil { return fmt.Errorf("open golden set: %w", err) }
	defer goldenFile.Close()
	golden, err := quality.LoadGolden(goldenFile)
	if err != nil { return err }

	thresholdFile, err := os.Open(thresholdPath)
	if err != nil { return fmt.Errorf("open quality thresholds: %w", err) }
	defer thresholdFile.Close()
	thresholds, err := quality.LoadThresholds(thresholdFile)
	if err != nil { return err }

	backend, err := searchbackend.New(searchbackend.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort)})
	if err != nil { return fmt.Errorf("create quality search backend: %w", err) }
	service := &searchsvc.Service{Backend: backend}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	report, err := quality.EvaluateGolden(ctx, service, golden)
	if err != nil { return err }
	gate := quality.CheckGate(report.Summary, thresholds)
	output := struct {
		Report     quality.Report     `json:"report"`
		Thresholds quality.Thresholds `json:"thresholds"`
		Gate       quality.GateResult `json:"gate"`
	}{Report: report, Thresholds: thresholds, Gate: gate}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil { return fmt.Errorf("encode quality report: %w", err) }
	if !gate.Pass { return errors.New("search quality gate failed") }
	return nil
}
