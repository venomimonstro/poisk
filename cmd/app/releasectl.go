package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	platformrelease "github.com/venomimonstro/poisk/internal/platform/release"
)

func releaseActor()string{if value:=strings.TrimSpace(os.Getenv("RELEASE_ACTOR"));value!=""{return value};return "releasectl"}

func runReleaseCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: releasectl stage|preflight|activate|rollback|current")}
	repo:=platformrelease.Repository{DB:pool};actor:=releaseActor()
	switch args[0]{
	case "stage":
		if len(args)!=8{return errors.New("usage: releasectl stage <version> <build-sha> <required-schema> <map-version|-> <config-sha256> <backend-image> <frontend-image>")}
		schema,err:=strconv.ParseInt(args[3],10,64);if err!=nil||schema<=0{return errors.New("invalid required schema")};mapVersion:=args[4];if mapVersion=="-"{mapVersion=""};manifest,err:=repo.Stage(ctx,args[1],args[2],schema,mapVersion,args[5],args[6],args[7],actor);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(manifest)
	case "preflight":
		if len(args)!=2{return errors.New("usage: releasectl preflight <version>")};result,err:=repo.Preflight(ctx,args[1],actor);_ = json.NewEncoder(os.Stdout).Encode(result);return err
	case "activate":
		if len(args)!=2{return errors.New("usage: releasectl activate <version>")};return repo.Activate(ctx,args[1],actor)
	case "rollback":
		if len(args)!=1{return errors.New("usage: releasectl rollback")};return repo.Rollback(ctx,actor)
	case "current":
		active,previous,err:=repo.Current(ctx);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(map[string]any{"active":active,"previous":previous})
	default:return errors.New("unknown releasectl command")
	}
}
