package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
	"github.com/venomimonstro/poisk/internal/readiness"
)

func runReadinessCtl(ctx context.Context,cfg config.Config,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: readinessctl check | record <type> <PASS|FAIL> <artifact_ref> <artifact_sha256> <actor> [details_json]")}
	versions,err:=migrate.ExpectedVersions(cfg.MigrationsDir);if err!=nil{return err}
	commit:=strings.ToLower(strings.TrimSpace(os.Getenv("READINESS_GIT_SHA")));if commit==""{return errors.New("READINESS_GIT_SHA is required and must be the exact 40-character commit under test")}
	switch args[0]{
	case "check":
		gate:=readiness.Gate{DB:pool,ExpectedVersions:versions,GitCommit:commit,InternetMail:internetMailEnabled()}
		report,err:=gate.Evaluate(ctx);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err=enc.Encode(report);err!=nil{return err};if !report.Ready{return errors.New("commercial readiness gate failed")};return nil
	case "record":
		if len(args)<6{return errors.New("usage: readinessctl record <type> <PASS|FAIL> <artifact_ref> <artifact_sha256> <actor> [details_json]")}
		details:=map[string]any{};if len(args)>6{if err:=json.Unmarshal([]byte(args[6]),&details);err!=nil{return fmt.Errorf("invalid details_json: %w",err)}}
		id,err:=readiness.RecordEvidence(ctx,pool,readiness.EvidenceInput{Type:args[1],Status:args[2],GitCommit:commit,DatabaseSchema:versions[len(versions)-1],ArtifactRef:args[3],ArtifactSHA256:args[4],Actor:args[5],Details:details,CompletedAt:time.Now().UTC()});if err!=nil{return err};fmt.Fprintf(os.Stdout,"evidence_id=%d\n",id);return nil
	default:return fmt.Errorf("readinessctl command %q is not implemented",args[0])
	}
}
