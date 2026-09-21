package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/billing"
	"github.com/venomimonstro/poisk/internal/growth/agency"
	agencyhttp "github.com/venomimonstro/poisk/internal/growth/agency/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

func registerAgencyRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,auth *webmaster.Service){
	h:=agencyhttp.Handler{Auth:auth,Agencies:agency.NewRepository(pool),Billing:billing.NewRepository(pool)}
	router.Mount("/api/agency",apiGuard.Protect(h.Routes()))
}
