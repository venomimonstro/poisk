package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	adminhttp "github.com/venomimonstro/poisk/internal/admin/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerAdminRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool)error{
	secureCookies:=!strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")),"development")
	service,err:=newAdminService(pool)
	if err!=nil{
		slog.Warn("admin API disabled; fail-closed", "reason", err.Error())
		handler:=adminhttp.Handler{Service:nil,SecureCookies:secureCookies}
		router.Mount("/api/admin",apiGuard.Protect(handler.Routes()))
		return nil
	}
	handler:=adminhttp.Handler{Service:service,SecureCookies:secureCookies}
	router.Mount("/api/admin",apiGuard.Protect(handler.Routes()))
	return nil
}
