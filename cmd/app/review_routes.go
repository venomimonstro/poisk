package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/identity"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	"github.com/venomimonstro/poisk/internal/reviews"
	reviewshttp "github.com/venomimonstro/poisk/internal/reviews/httpapi"
)

func registerReviewRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool){
	identityService:=&identity.Service{Repo:identity.NewRepository(pool)}
	handler:=reviewshttp.Handler{Repo:reviews.Repository{DB:pool},Identity:identityService}
	router.Mount("/api/reviews",apiGuard.Protect(handler.Routes()))
}
