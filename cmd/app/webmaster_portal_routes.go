package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/billing"
	"github.com/venomimonstro/poisk/internal/identity"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/webmaster"
	webmasterhttp "github.com/venomimonstro/poisk/internal/webmaster/httpapi"
)

func registerWebmasterPortalRoutes(router chi.Router, apiGuard guard.Middleware, pool *pgxpool.Pool, service *webmaster.Service, billingRepo *billing.Repository) {
	identityService := &identity.Service{Repo: identity.NewRepository(pool)}
	base := webmasterhttp.Handler{Service: service, Billing: billingRepo}
	portal := webmasterhttp.PortalHandler{Base: base, Identity: identityService}
	router.Mount("/api/portal/webmaster", apiGuard.Protect(portal.Routes()))
}
