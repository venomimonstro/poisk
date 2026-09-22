package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/datahub"
	datahubhttp "github.com/venomimonstro/poisk/internal/datahub/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerDataHubRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool){
	h:=datahubhttp.Handler{Hub:datahub.NewRepository(pool)}
	router.Mount("/api/datahub",apiGuard.Protect(h.Routes()))
}
