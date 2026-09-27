package main

import (
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/identity"
	identityhttp "github.com/venomimonstro/poisk/internal/identity/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func browserSecureCookies(env string)bool{
	switch strings.ToLower(strings.TrimSpace(env)){case "dev","development","local","test":return false;default:return true}
}

func registerAccountRoutes(router chi.Router,apiGuard guard.Middleware,cfg config.Config,pool *pgxpool.Pool){
	repo:=identity.NewRepository(pool)
	service:=&identity.Service{Repo:repo}
	handler:=identityhttp.Handler{Service:service,SecureCookies:browserSecureCookies(cfg.Env)}
	router.Mount("/api/account",apiGuard.Protect(handler.Routes()))
	registerReviewRoutes(router,apiGuard,pool)
	registerMailRoutes(router,apiGuard,pool,service)
}
