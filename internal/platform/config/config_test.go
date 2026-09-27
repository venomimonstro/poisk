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

func TestLoadProductionRequiresHTTPSPublicOrigin(t *testing.T){
	t.Setenv("POSTGRES_PASSWORD","secret");t.Setenv("APP_ENV","production");t.Setenv("PUBLIC_BASE_URL","")
	if _,err:=Load();err==nil{t.Fatal("expected missing public HTTPS origin rejection")}
	t.Setenv("PUBLIC_BASE_URL","http://search.example.test")
	if _,err:=Load();err==nil{t.Fatal("expected HTTP public origin rejection")}
	t.Setenv("PUBLIC_BASE_URL","https://search.example.test/app")
	if _,err:=Load();err==nil{t.Fatal("expected public origin path rejection")}
	t.Setenv("PUBLIC_BASE_URL","https://search.example.test/")
	cfg,err:=Load();if err!=nil{t.Fatalf("Load() error = %v",err)};if cfg.PublicBaseURL!="https://search.example.test"{t.Fatalf("public base=%q",cfg.PublicBaseURL)}
}

func TestLoadUnknownNonLocalEnvironmentAlsoRequiresHTTPSOrigin(t *testing.T){
	t.Setenv("POSTGRES_PASSWORD","secret");t.Setenv("APP_ENV","qa");t.Setenv("PUBLIC_BASE_URL","")
	if _,err:=Load();err==nil{t.Fatal("expected unknown non-local environment to require public HTTPS origin")}
	t.Setenv("PUBLIC_BASE_URL","https://qa.example.test")
	if _,err:=Load();err!=nil{t.Fatalf("Load() error = %v",err)}
}

func TestLoadInternetMailUsesCanonicalVariables(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("MAIL_INTERNET_ENABLED", "true")
	t.Setenv("MAIL_DOMAIN", "mail.example.test")
	t.Setenv("MAIL_MTA_BASE_URL", "https://mta-bridge.internal:8443")
	t.Setenv("MAIL_GATEWAY_SHARED_SECRET", "0123456789abcdef0123456789abcdef")
	cfg, err := Load()
	if err != nil { t.Fatalf("Load() error = %v", err) }
	if !cfg.MailInternetEnabled || cfg.MailDomain != "mail.example.test" || cfg.MailMTABaseURL != "https://mta-bridge.internal:8443" {
		t.Fatalf("unexpected mail config: %+v", cfg)
	}
}

func TestLoadInternetMailRejectsHTTPMTA(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("MAIL_INTERNET_ENABLED", "true")
	t.Setenv("MAIL_DOMAIN", "mail.example.test")
	t.Setenv("MAIL_MTA_BASE_URL", "http://mta-bridge.internal:8080")
	t.Setenv("MAIL_GATEWAY_SHARED_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load(); err == nil { t.Fatal("expected insecure MTA URL rejection") }
}

func TestLoadInternetMailRejectsShortSecret(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("MAIL_INTERNET_ENABLED", "true")
	t.Setenv("MAIL_DOMAIN", "mail.example.test")
	t.Setenv("MAIL_MTA_BASE_URL", "https://mta-bridge.internal:8443")
	t.Setenv("MAIL_GATEWAY_SHARED_SECRET", "too-short")
	if _, err := Load(); err == nil { t.Fatal("expected short gateway secret rejection") }
}

func TestLoadInternetMailDoesNotAcceptLegacyVariableNames(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("MAIL_INTERNET_ENABLED", "true")
	t.Setenv("MAIL_PUBLIC_DOMAIN", "legacy.example.test")
	t.Setenv("MAIL_RELAY_ADDR", "legacy.example.test:25")
	t.Setenv("MAIL_GATEWAY_SECRET_FILE", "/run/secrets/legacy")
	t.Setenv("MAIL_DOMAIN", "")
	t.Setenv("MAIL_MTA_BASE_URL", "")
	t.Setenv("MAIL_GATEWAY_SHARED_SECRET", "")
	if _, err := Load(); err == nil { t.Fatal("expected canonical Internet Mail variables to be required") }
}
