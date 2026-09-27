package main

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
	"github.com/venomimonstro/poisk/internal/readiness"
)

type adminReadinessReader struct{
	db *pgxpool.Pool
	cfg config.Config
}

func (r adminReadinessReader) Evaluate(ctx context.Context)(readiness.Report,error){
	if r.db==nil{return readiness.Report{},errors.New("readiness database is not initialized")}
	versions,err:=migrate.ExpectedVersions(r.cfg.MigrationsDir);if err!=nil{return readiness.Report{},err}
	commit:=strings.ToLower(strings.TrimSpace(os.Getenv("READINESS_GIT_SHA")))
	releaseVersion:=strings.TrimSpace(os.Getenv("RELEASE_VERSION"))
	if commit==""||releaseVersion==""||releaseVersion=="dev"{return readiness.Report{},errors.New("READINESS_GIT_SHA and a staged RELEASE_VERSION are required")}
	gate:=readiness.Gate{DB:r.db,ExpectedVersions:versions,GitCommit:commit,ReleaseVersion:releaseVersion,InternetMail:r.cfg.MailInternetEnabled,MailDomain:r.cfg.MailDomain,MailSelector:strings.TrimSpace(os.Getenv("MAIL_DKIM_SELECTOR"))}
	return gate.Evaluate(ctx)
}
