package main

import (
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/address"
	addressbackend "github.com/venomimonstro/poisk/internal/address/backend"
	addresshttp "github.com/venomimonstro/poisk/internal/address/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerAddressRoutes(router chi.Router,apiGuard guard.Middleware,cfg config.Config,pool *pgxpool.Pool)error{
	backend,err:=addressbackend.New(addressbackend.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort),MaxResults:50});if err!=nil{return fmt.Errorf("create address backend: %w",err)}
	service:=&address.SearchService{Backend:backend,DB:pool};handler:=addresshttp.Handler{Service:service}
	router.With(apiGuard.Protect).Get("/api/address/search",handler.Search)
	router.With(apiGuard.Protect).Get("/api/address/geocode",handler.Forward)
	router.With(apiGuard.Protect).Get("/api/address/reverse",handler.Reverse)
	return nil
}
