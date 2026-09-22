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

	answersvc "github.com/venomimonstro/poisk/internal/answer"
	answerhttp "github.com/venomimonstro/poisk/internal/answer/httpapi"
	"github.com/venomimonstro/poisk/internal/billing"
	crawlerfetcher "github.com/venomimonstro/poisk/internal/crawler/fetcher"
	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
	"github.com/venomimonstro/poisk/internal/demand"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
	"github.com/venomimonstro/poisk/internal/indexer/source"
	indexworker "github.com/venomimonstro/poisk/internal/indexer/worker"
	maphttp "github.com/venomimonstro/poisk/internal/maps/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/platform/health"
	platformmetrics "github.com/venomimonstro/poisk/internal/platform/metrics"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
	"github.com/venomimonstro/poisk/internal/quality"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
	searchhttp "github.com/venomimonstro/poisk/internal/search/httpapi"
	"github.com/venomimonstro/poisk/internal/webmaster"
	webmasterhttp "github.com/venomimonstro/poisk/internal/webmaster/httpapi"
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
	case "webmaster-worker":
		return runWebmasterWorker(pool)
	case "organizations-worker":
		return runOrganizationsWorker(pool)
	case "organization-indexer":
		return runOrganizationIndexer(cfg, pool)
	case "address-indexer":
		return runAddressIndexer(cfg, pool)
	case "resource-monitor":
		return runResourceMonitor(pool)
	case "demand-worker":
		return runDemandWorker(pool)
	case "datahub-worker":
		return runDataHubWorker(pool)
	case "demandctl":
		return runDemandCtl(ctx, pool, os.Args[2:])
	case "datahubctl":
		return runDataHubCtl(ctx, pool, os.Args[2:])
	case "capacityctl":
		return runCapacityCtl(ctx, pool, os.Args[2:])
	case "mapctl":
		return runMapCtl(ctx, pool, os.Args[2:])
	case "orgctl":
		return runOrgCtl(ctx, pool, os.Args[2:])
	case "addressctl":
		return runAddressCtl(ctx, cfg, pool, os.Args[2:])
	case "indexctl":
		return runIndexCtl(ctx, cfg, pool, os.Args[2:])
	case "adminctl":
		return runAdminCtl(ctx, pool, os.Args[2:])
	case "releasectl":
		return runReleaseCtl(ctx, pool, os.Args[2:])
	case "billingctl":
		return runBillingCtl(ctx, pool, os.Args[2:])
	case "quality":
		return runQuality(cfg)
	default:
		return fmt.Errorf("runtime mode %q is not implemented in current sprint", mode)
	}
}

func runAPI(cfg config.Config, pool *pgxpool.Pool) error {
	checker := health.Checker{DB: pool, ManticoreHost: cfg.ManticoreHost, ManticoreSQLPort: cfg.ManticoreSQLPort}
	webmasterRepo := webmaster.NewRepository(pool)
	billingRepo := billing.NewRepository(pool)
	demandRepo := demand.NewRepository(pool)
	searchBackend, err := searchbackend.New(searchbackend.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort)})
	if err != nil { return fmt.Errorf("create search backend: %w", err) }
	searchService := &searchsvc.Service{Backend: searchBackend, Cache: searchsvc.NewCache(512), BackendConcurrency: make(chan struct{}, cfg.BackendConcurrent)}
	searchHandler := searchhttp.Handler{SearchService: searchService, Impressions: webmasterRepo, Demand: demandRepo}
	answerService := &answersvc.Service{Search: searchService, MinConfidence: answersvc.DefaultMinConfidence}
	answerHandler := answerhttp.Handler{AnswerService: answerService, Citations: webmasterRepo}
	trackingHandler := webmasterhttp.TrackingHandler{Recorder: webmasterRepo}
	mapHandler := maphttp.Handler{Maps: newMapService(pool)}

	validator := crawlersecurity.NewValidator()
	proofCfg := crawlerfetcher.DefaultConfig()
	proofCfg.UserAgent = "PoiskWebmasterVerifier/1.0"
	proofCfg.MaxBodyBytes = 256 << 10
	proofCfg.MaxRedirects = 3
	proofCfg.MaxRetries = 1
	proofCfg.RequestTimeout = 5 * time.Second
	proofCfg.ResponseHeaderTimeout = 3 * time.Second
	proofFetcher := crawlerfetcher.New(proofCfg, validator)
	defer proofFetcher.CloseIdleConnections()
	webmasterService := &webmaster.Service{Store:webmasterRepo,Validator:validator,Verifier:webmaster.Verifier{Fetcher:proofFetcher},SessionTTL:7*24*time.Hour,VerificationTTL:30*time.Minute}
	webmasterHandler := webmasterhttp.Handler{Service:webmasterService,Billing:billingRepo}

	latencyRecorder := platformmetrics.NewLatencyRecorder(4096)
	apiLimiter := guard.NewLimiter(cfg.APIRatePerSecond, cfg.APIRateBurst, cfg.APIRateClients, cfg.APIRateIdleTTL)
	apiGuard := guard.NewMiddleware(apiLimiter, cfg.APIRequestConcurrent, cfg.APIRequestDeadline)
	apiGuard.Latency = latencyRecorder

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Get("/health/live", checker.Live)
	router.Get("/health/ready", checker.Ready)
	router.Get("/health/perf", perfHandler(latencyRecorder))
	router.With(apiGuard.Protect).Get("/api/search", searchHandler.Search)
	router.With(apiGuard.Protect).Get("/api/answer", answerHandler.Answer)
	router.With(apiGuard.Protect).Get("/api/map/config", mapHandler.Config)
	router.With(apiGuard.Protect).Post("/api/click", trackingHandler.Click)
	registerAccountRoutes(router,apiGuard,cfg,pool)
	registerWebmasterPortalRoutes(router,apiGuard,pool,webmasterService,billingRepo)
	router.Mount("/api/webmaster", apiGuard.Protect(webmasterHandler.Routes()))
	registerBillingRoutes(router,apiGuard,pool,webmasterService)
	if err:=registerGeoRoute(router,apiGuard,cfg);err!=nil{return err}
	if err:=registerAddressRoutes(router,apiGuard,cfg,pool);err!=nil{return err}
	if err:=registerAdminRoutes(router,apiGuard,pool);err!=nil{return err}
	if err:=registerWidgetRoutes(router,apiGuard,cfg,pool);err!=nil{return err}
	registerDataHubRoutes(router,apiGuard,pool)

	server := &http.Server{Addr:cfg.Addr,Handler:router,ReadHeaderTimeout:3*time.Second,ReadTimeout:5*time.Second,WriteTimeout:5*time.Second,IdleTimeout:60*time.Second}
	serverErr := make(chan error, 1)
	go func() { slog.Info("http server started", "addr", cfg.Addr, "env", cfg.Env, "mode", "api"); serverErr <- server.ListenAndServe() }()
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-sigCtx.Done(): slog.Info("shutdown signal received")
	case err := <-serverErr: if err != nil && !errors.Is(err, http.ErrServerClosed) { return err }
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func perfHandler(recorder *platformmetrics.LatencyRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := recorder.Snapshot()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_ = json.NewEncoder(w).Encode(map[string]any{"count":s.Count,"p50_ms":float64(s.P50.Microseconds())/1000,"p95_ms":float64(s.P95.Microseconds())/1000,"p99_ms":float64(s.P99.Microseconds())/1000})
	}
}

func runIndexer(cfg config.Config, pool *pgxpool.Pool) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	index, err := indexmanticore.New(indexmanticore.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort)})
	if err != nil { return fmt.Errorf("create search index client: %w", err) }
	if err := index.EnsureSchema(ctx); err != nil { return fmt.Errorf("ensure web index schema: %w", err) }
	outboxRepo := outbox.NewRepositoryForEntityTypes(pool, indexworker.WebDocumentEntity)
	sourceRepo := source.NewRepository(pool)
	processor := &indexworker.Processor{Source:sourceRepo,Index:index,Ack:outboxRepo,RetryBase:time.Second,RetryMax:time.Minute}
	runner := indexworker.Runner{Leases:outboxRepo,Processor:processor,WorkerID:fmt.Sprintf("indexer-%d",os.Getpid()),BatchSize:32,LeaseSeconds:30,PollInterval:500*time.Millisecond}
	slog.Info("indexer worker started", "worker_id", runner.WorkerID, "entity_type", indexworker.WebDocumentEntity)
	return runner.Run(ctx)
}

func runWebmasterWorker(pool *pgxpool.Pool) error {
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM)
	defer stop()
	validator:=crawlersecurity.NewValidator()
	fetchCfg:=crawlerfetcher.DefaultConfig()
	fetchCfg.UserAgent="PoiskWebmasterSitemap/1.0"
	fetchCfg.MaxBodyBytes=8<<20
	fetchCfg.MaxRedirects=5
	fetchCfg.MaxRetries=2
	fetchCfg.RequestTimeout=20*time.Second
	fetcher:=crawlerfetcher.New(fetchCfg,validator)
	defer fetcher.CloseIdleConnections()
	repo:=webmaster.NewRepository(pool)
	processor:=webmaster.SitemapProcessor{Store:repo,Fetcher:fetcher}
	runner:=webmaster.SitemapRunner{Store:repo,Processor:processor,WorkerID:fmt.Sprintf("webmaster-%d",os.Getpid()),BatchSize:4,LeaseSeconds:45,PollInterval:time.Second}
	slog.Info("webmaster sitemap worker started","worker_id",runner.WorkerID)
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
	service := &searchsvc.Service{Backend: backend, BackendConcurrency: make(chan struct{}, cfg.BackendConcurrent)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	report, err := quality.EvaluateGolden(ctx, service, golden)
	if err != nil { return err }
	gate := quality.CheckGate(report.Summary, thresholds)
	output := struct { Report quality.Report `json:"report"`; Thresholds quality.Thresholds `json:"thresholds"`; Gate quality.GateResult `json:"gate"` }{Report:report,Thresholds:thresholds,Gate:gate}
	encoder := json.NewEncoder(os.Stdout); encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil { return fmt.Errorf("encode quality report: %w", err) }
	if !gate.Pass { return errors.New("search quality gate failed") }
	return nil
}
