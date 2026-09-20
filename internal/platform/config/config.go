package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                  string
	Addr                 string
	ShutdownTimeout      time.Duration
	LogLevel             string
	PostgresDSN          string
	ManticoreHost        string
	ManticoreSQLPort     int
	ManticoreHTTPPort    int
	MigrationsDir        string
	APIRequestDeadline   time.Duration
	APIRequestConcurrent int
	BackendConcurrent    int
	APIRatePerSecond     float64
	APIRateBurst         int
	APIRateClients       int
	APIRateIdleTTL       time.Duration
	MapArtifactRoot      string
	MapPublicPrefix      string
}

func Load() (Config, error) {
	cfg := Config{
		Env:             getenv("APP_ENV", "development"),
		Addr:            getenv("APP_ADDR", ":8080"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		ManticoreHost:   getenv("MANTICORE_HOST", "manticore"),
		MigrationsDir:   getenv("MIGRATIONS_DIR", "/app/db/migrations"),
		MapArtifactRoot: getenv("MAP_ARTIFACT_ROOT", "/srv/maps"),
		MapPublicPrefix: getenv("MAP_PUBLIC_PREFIX", "/maps"),
	}

	shutdown, err := positiveDuration("APP_SHUTDOWN_TIMEOUT", "10s")
	if err != nil { return Config{}, err }
	cfg.ShutdownTimeout = shutdown

	deadline, err := positiveDuration("API_REQUEST_DEADLINE", "3s")
	if err != nil { return Config{}, err }
	cfg.APIRequestDeadline = deadline

	idleTTL, err := positiveDuration("API_RATE_IDLE_TTL", "10m")
	if err != nil { return Config{}, err }
	cfg.APIRateIdleTTL = idleTTL

	port, err := portValue("MANTICORE_SQL_PORT", "9306")
	if err != nil { return Config{}, err }
	cfg.ManticoreSQLPort = port

	httpPort, err := portValue("MANTICORE_HTTP_PORT", "9308")
	if err != nil { return Config{}, err }
	cfg.ManticoreHTTPPort = httpPort

	if cfg.APIRequestConcurrent, err = positiveInt("API_MAX_CONCURRENT", "64"); err != nil { return Config{}, err }
	if cfg.BackendConcurrent, err = positiveInt("SEARCH_BACKEND_MAX_CONCURRENT", "32"); err != nil { return Config{}, err }
	if cfg.APIRateBurst, err = positiveInt("API_RATE_BURST", "20"); err != nil { return Config{}, err }
	if cfg.APIRateClients, err = positiveInt("API_RATE_MAX_CLIENTS", "10000"); err != nil { return Config{}, err }

	rate, err := strconv.ParseFloat(getenv("API_RATE_PER_SECOND", "10"), 64)
	if err != nil || rate <= 0 || rate > 100000 { return Config{}, fmt.Errorf("invalid API_RATE_PER_SECOND") }
	cfg.APIRatePerSecond = rate

	if strings.TrimSpace(cfg.MapArtifactRoot)=="" { return Config{}, errors.New("MAP_ARTIFACT_ROOT is required") }
	if !strings.HasPrefix(cfg.MapPublicPrefix,"/") || strings.Contains(cfg.MapPublicPrefix,"..") || strings.ContainsAny(cfg.MapPublicPrefix,"?#") { return Config{}, errors.New("invalid MAP_PUBLIC_PREFIX") }

	db := getenv("POSTGRES_DB", "poisk")
	user := getenv("POSTGRES_USER", "poisk")
	password := os.Getenv("POSTGRES_PASSWORD")
	host := getenv("POSTGRES_HOST", "postgres")
	pgPort := getenv("POSTGRES_PORT", "5432")
	sslMode := getenv("POSTGRES_SSLMODE", "disable")

	if password == "" { return Config{}, errors.New("POSTGRES_PASSWORD is required") }
	if cfg.MigrationsDir == "" { return Config{}, errors.New("MIGRATIONS_DIR is required") }

	cfg.PostgresDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, pgPort, db, sslMode)
	return cfg, nil
}

func positiveDuration(key, fallback string) (time.Duration, error) {
	v, err := time.ParseDuration(getenv(key, fallback))
	if err != nil || v <= 0 { return 0, fmt.Errorf("invalid %s", key) }
	return v, nil
}

func positiveInt(key, fallback string) (int, error) {
	v, err := strconv.Atoi(getenv(key, fallback))
	if err != nil || v <= 0 { return 0, fmt.Errorf("invalid %s", key) }
	return v, nil
}

func portValue(key, fallback string) (int, error) {
	v, err := strconv.Atoi(getenv(key, fallback))
	if err != nil || v <= 0 || v > 65535 { return 0, fmt.Errorf("invalid %s", key) }
	return v, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}
