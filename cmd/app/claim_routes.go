package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/billing"
	"github.com/venomimonstro/poisk/internal/growth/claim"
	claimhttp "github.com/venomimonstro/poisk/internal/growth/claim/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

func registerClaimRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,auth *webmaster.Service){
	h:=claimhttp.Handler{Auth:auth,Claims:claim.NewRepository(pool),Billing:billing.NewRepository(pool)}
	router.Mount("/api/claims",apiGuard.Protect(h.Routes()))
}
