package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/admin"
	adminhttp "github.com/venomimonstro/poisk/internal/admin/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerAdminRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool)error{
	env:=strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	secureCookies:=env!="development"&&env!="dev"
	service,err:=newAdminService(pool)
	if err!=nil{
		slog.Warn("admin API disabled; fail-closed", "reason", err.Error())
		handler:=adminhttp.Handler{Service:nil,Ops:nil,Owner:nil,Diagnostics:nil,SecureCookies:secureCookies}
		router.Mount("/api/admin",apiGuard.Protect(handler.Routes()))
		return nil
	}
	handler:=adminhttp.Handler{
		Service:service,
		Ops:admin.OpsRepository{DB:pool},
		Owner:admin.OwnerRepository{DB:pool},
		Diagnostics:admin.DiagnosticsRepository{DB:pool},
		SecureCookies:secureCookies,
	}
	router.Mount("/api/admin",apiGuard.Protect(handler.Routes()))
	return nil
}
