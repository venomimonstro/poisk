package main

import (
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/identity"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
	gatewayhttp "github.com/venomimonstro/poisk/internal/mail/gatewayhttp"
	mailhttp "github.com/venomimonstro/poisk/internal/mail/httpapi"
	internethttp "github.com/venomimonstro/poisk/internal/mail/internethttp"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func internetMailEnabled()bool{v:=strings.ToLower(strings.TrimSpace(os.Getenv("MAIL_INTERNET_ENABLED")));return v=="true"||v=="1"||v=="yes"}

func registerMailRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,identityService *identity.Service){
	root:=strings.TrimSpace(os.Getenv("MAIL_BLOB_DIR"));if root==""{root="/mail-blobs"}
	repo:=mailcore.Repository{DB:pool};store:=mailcore.AttachmentStore{Repo:repo,Root:root}
	handler:=mailhttp.Handler{Repo:repo,Store:store,Identity:identityService}
	router.Mount("/api/mail",apiGuard.Protect(handler.Routes()))
	internet:=internethttp.Handler{Repo:repo,Identity:identityService,Enabled:internetMailEnabled(),Domain:strings.ToLower(strings.TrimSpace(os.Getenv("MAIL_DOMAIN")))}
	router.Mount("/api/mail/internet",apiGuard.Protect(internet.Routes()))
	gateway:=gatewayhttp.Handler{Repo:repo,Inbound:mailcore.InboundStore{Repo:repo,Root:root},Secret:[]byte(strings.TrimSpace(os.Getenv("MAIL_GATEWAY_SHARED_SECRET"))),Enabled:internetMailEnabled()}
	router.Mount("/internal/mail-gateway",apiGuard.Protect(gateway.Routes()))
}
