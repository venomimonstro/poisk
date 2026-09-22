package main

import (
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/identity"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
	mailhttp "github.com/venomimonstro/poisk/internal/mail/httpapi"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

func registerMailRoutes(router chi.Router,apiGuard guard.Middleware,pool *pgxpool.Pool,identityService *identity.Service){
	root:=strings.TrimSpace(os.Getenv("MAIL_BLOB_DIR"));if root==""{root="/mail-blobs"}
	repo:=mailcore.Repository{DB:pool};store:=mailcore.AttachmentStore{Repo:repo,Root:root}
	handler:=mailhttp.Handler{Repo:repo,Store:store,Identity:identityService}
	router.Mount("/api/mail",apiGuard.Protect(handler.Routes()))
}
