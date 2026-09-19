package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env               string
	Addr              string
	ShutdownTimeout   time.Duration
	LogLevel          string
	PostgresDSN       string
	ManticoreHost     string
	ManticoreSQLPort  int
	ManticoreHTTPPort int
	MigrationsDir     string
}

func Load() (Config, error) {
	cfg := Config{
		Env:           getenv("APP_ENV", "development"),
		Addr:          getenv("APP_ADDR", ":8080"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		ManticoreHost: getenv("MANTICORE_HOST", "manticore"),
		MigrationsDir: getenv("MIGRATIONS_DIR", "/app/db/migrations"),
	}

	shutdown, err := time.ParseDuration(getenv("APP_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || shutdown <= 0 {
		return Config{}, fmt.Errorf("invalid APP_SHUTDOWN_TIMEOUT")
	}
	cfg.ShutdownTimeout = shutdown

	port, err := strconv.Atoi(getenv("MANTICORE_SQL_PORT", "9306"))
	if err != nil || port <= 0 || port > 65535 {
		return Config{}, fmt.Errorf("invalid MANTICORE_SQL_PORT")
	}
	cfg.ManticoreSQLPort = port

	httpPort, err := strconv.Atoi(getenv("MANTICORE_HTTP_PORT", "9308"))
	if err != nil || httpPort <= 0 || httpPort > 65535 {
		return Config{}, fmt.Errorf("invalid MANTICORE_HTTP_PORT")
	}
	cfg.ManticoreHTTPPort = httpPort

	db := getenv("POSTGRES_DB", "poisk")
	user := getenv("POSTGRES_USER", "poisk")
	password := os.Getenv("POSTGRES_PASSWORD")
	host := getenv("POSTGRES_HOST", "postgres")
	pgPort := getenv("POSTGRES_PORT", "5432")
	sslMode := getenv("POSTGRES_SSLMODE", "disable")

	if password == "" {
		return Config{}, errors.New("POSTGRES_PASSWORD is required")
	}
	if cfg.MigrationsDir == "" {
		return Config{}, errors.New("MIGRATIONS_DIR is required")
	}

	cfg.PostgresDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, pgPort, db, sslMode)
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
