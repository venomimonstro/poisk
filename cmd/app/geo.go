package main

import (
	"fmt"

	"github.com/go-chi/chi/v5"
	geosvc "github.com/venomimonstro/poisk/internal/geo"
	geobackend "github.com/venomimonstro/poisk/internal/geo/backend"
	geohttp "github.com/venomimonstro/poisk/internal/geo/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerGeoRoute(router chi.Router, apiGuard guard.Middleware, cfg config.Config) error {
	backend, err := geobackend.New(geobackend.Config{BaseURL: fmt.Sprintf("http://%s:%d", cfg.ManticoreHost, cfg.ManticoreHTTPPort), MaxResults: 200})
	if err != nil { return fmt.Errorf("create geo backend: %w", err) }
	service := &geosvc.Service{Backend: backend, MaxCandidates: 200}
	handler := geohttp.Handler{Service: service}
	router.With(apiGuard.Protect).Get("/api/geo/search", handler.Search)
	router.With(apiGuard.Protect).Get("/api/geo/viewport", handler.Viewport)
	return nil
}
