package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/billing"
	billinghttp "github.com/venomimonstro/poisk/internal/billing/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

func registerBillingRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,auth *webmaster.Service){
	h:=billinghttp.Handler{Auth:auth,Billing:billing.NewRepository(pool)}
	router.Mount("/api/billing",apiGuard.Protect(h.Routes()))
}
