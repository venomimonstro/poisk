package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/growth/referral"
	referralhttp "github.com/venomimonstro/poisk/internal/growth/referral/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

func registerReferralRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,auth *webmaster.Service){
	h:=referralhttp.Handler{Auth:auth,Referrals:referral.NewRepository(pool)}
	router.Mount("/api/growth/attribution",apiGuard.Protect(h.PublicRoutes()))
	router.Mount("/api/growth/referrals",apiGuard.Protect(h.OwnerRoutes()))
}
