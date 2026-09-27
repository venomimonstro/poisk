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
	commit:=strings.ToLower(strings.TrimSpace(os.Getenv("POISK_GIT_COMMIT")))
	releaseVersion:=strings.TrimSpace(os.Getenv("POISK_RELEASE_VERSION"))
	if commit==""||releaseVersion==""{return readiness.Report{},errors.New("POISK_GIT_COMMIT and POISK_RELEASE_VERSION are required")}
	internetMail:=strings.EqualFold(strings.TrimSpace(os.Getenv("POISK_INTERNET_MAIL_ENABLED")),"true")
	gate:=readiness.Gate{DB:r.db,ExpectedVersions:versions,GitCommit:commit,ReleaseVersion:releaseVersion,InternetMail:internetMail,MailDomain:strings.TrimSpace(os.Getenv("POISK_MAIL_DOMAIN")),MailSelector:strings.TrimSpace(os.Getenv("POISK_MAIL_DKIM_SELECTOR"))}
	return gate.Evaluate(ctx)
}
