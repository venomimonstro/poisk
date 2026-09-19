package main

import (
	"context"
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

	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/health"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
	searchhttp "github.com/venomimonstro/poisk/internal/search/httpapi"
)

func main() {
	if err := run(); err != nil {
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
	default:
		return fmt.Errorf("runtime mode %q is not implemented in current sprint", mode)
	}
}

func runAPI(cfg config.Config, pool *pgxpool.Pool) error {
	checker := health.Checker{DB: pool, ManticoreHost: cfg.ManticoreHost, ManticoreSQLPort: cfg.ManticoreSQLPort}

	searchBackend, err := searchbackend.New(searchbackend.Config{
		BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort),
	})
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
