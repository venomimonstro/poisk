package main

import (
	"log/slog"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/admin"
	adminhttp "github.com/venomimonstro/poisk/internal/admin/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerAdminRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool)error{
	secureCookies:=browserSecureCookies(os.Getenv("APP_ENV"))
	service,err:=newAdminService(pool)
	if err!=nil{
		slog.Warn("admin API disabled; fail-closed", "reason", err.Error())
		handler:=adminhttp.Handler{Service:nil,Ops:nil,Owner:nil,Diagnostics:nil,Readiness:nil,SecureCookies:secureCookies}
		router.Mount("/api/admin",apiGuard.Protect(handler.Routes()))
		return nil
	}
	var readinessReader adminhttp.ReadinessReader
	cfg,cfgErr:=config.Load()
	if cfgErr!=nil{
		slog.Warn("admin readiness view disabled; fail-closed", "reason", cfgErr.Error())
	}else{
		readinessReader=adminReadinessReader{db:pool,cfg:cfg}
	}
	handler:=adminhttp.Handler{
		Service:service,
		Ops:admin.OpsRepository{DB:pool},
		Owner:admin.OwnerRepository{DB:pool},
		Diagnostics:admin.DiagnosticsRepository{DB:pool},
		Readiness:readinessReader,
		SecureCookies:secureCookies,
	}
	router.Mount("/api/admin",apiGuard.Protect(handler.Routes()))
	return nil
}
