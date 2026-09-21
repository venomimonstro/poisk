package main

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/growth/widget"
	widgethttp "github.com/venomimonstro/poisk/internal/growth/widget/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/guard"
	searchbackend "github.com/venomimonstro/poisk/internal/search/backend"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

func registerWidgetRoutes(router chi.Router,apiGuard guard.Middleware,cfg config.Config,pool *pgxpool.Pool)error{
	backend,err:=searchbackend.New(searchbackend.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort),MaxResults:40});if err!=nil{return err}
	repo:=widget.NewRepository(pool)
	publicHandler:=widgethttp.Handler{
		Widgets:repo,
		Search:widget.Service{Backend:backend,Usage:repo},
		KeyLimiter:guard.NewLimiter(50,100,10000,10*time.Minute),
		ClientLimiter:guard.NewLimiter(5,10,50000,10*time.Minute),
	}
	webmasterAuth:=&webmaster.Service{Store:webmaster.NewRepository(pool)}
	manageHandler:=widgethttp.ManageHandler{Auth:webmasterAuth,Widgets:repo}
	router.Handle("/api/widget/search",apiGuard.Protect(publicHandler))
	router.Handle("/api/widget/manage",apiGuard.Protect(manageHandler))
	registerAgencyRoutes(router,apiGuard,pool,webmasterAuth)
	registerClaimRoutes(router,apiGuard,pool,webmasterAuth)
	return nil
}
