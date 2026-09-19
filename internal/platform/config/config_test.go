package config

import (
	"os"
	"testing"
)

func TestLoadRequiresPostgresPassword(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when POSTGRES_PASSWORD is empty")
	}
}

func TestLoadValidConfiguration(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("MANTICORE_SQL_PORT", "9306")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PostgresDSN == "" || cfg.MigrationsDir == "" {
		t.Fatalf("configuration is incomplete: %+v", cfg)
	}
	_ = os.Unsetenv("POSTGRES_PASSWORD")
}

func TestLoadRejectsInvalidManticorePort(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("MANTICORE_SQL_PORT", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid Manticore port error")
	}
}
