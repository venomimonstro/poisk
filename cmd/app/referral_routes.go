package main

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/growth/referral"
	referralhttp "github.com/venomimonstro/poisk/internal/growth/referral/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

func registerReferralRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,auth *webmaster.Service){
	h:=referralhttp.Handler{Auth:auth,Referrals:referral.NewRepository(pool)}
	publicGuard:=guard.NewMiddleware(guard.NewLimiter(2,5,50000,10*time.Minute),16,2*time.Second)
	router.Mount("/api/growth/attribution",apiGuard.Protect(publicGuard.Protect(h.PublicRoutes())))
	router.Mount("/api/growth/referrals",apiGuard.Protect(h.OwnerRoutes()))
}
