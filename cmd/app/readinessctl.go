package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/platform/config"
	"github.com/venomimonstro/poisk/internal/platform/migrate"
	"github.com/venomimonstro/poisk/internal/readiness"
)

const maxReadinessArtifactBytes int64 = 256 << 20

func runReadinessCtl(ctx context.Context,cfg config.Config,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: readinessctl candidate|check|fresh-install-db|upgrade-db | record <type> <PASS|FAIL> <artifact_ref> <artifact_sha256> <actor> [details_json] | record-file <type> <PASS|FAIL> <artifact_path> <actor> [details_json]")}
	versions,err:=migrate.ExpectedVersions(cfg.MigrationsDir);if err!=nil{return err}
	commit:=strings.ToLower(strings.TrimSpace(os.Getenv("READINESS_GIT_SHA")));if commit==""{return errors.New("READINESS_GIT_SHA is required and must be the exact 40-character commit under test")}
	releaseVersion:=strings.TrimSpace(os.Getenv("RELEASE_VERSION"))
	if releaseVersion==""||releaseVersion=="dev"{return errors.New("RELEASE_VERSION is required and must identify the staged release candidate")}
	latestSchema:=versions[len(versions)-1]
	gate:=readiness.Gate{DB:pool,ExpectedVersions:versions,GitCommit:commit,ReleaseVersion:releaseVersion,InternetMail:cfg.MailInternetEnabled,MailDomain:cfg.MailDomain,MailSelector:strings.TrimSpace(os.Getenv("MAIL_DKIM_SELECTOR"))}
	verifyCurrentDB:=func()error{return migrate.VerifyAppliedChecksums(ctx,pool,cfg.MigrationsDir)}
	switch args[0]{
	case "candidate":
		if err:=verifyCurrentDB();err!=nil{return fmt.Errorf("migration integrity gate failed: %w",err)}
		report,err:=gate.Candidate(ctx);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err=enc.Encode(report);err!=nil{return err};if !report.Valid{return errors.New("release candidate precheck failed")};return nil
	case "check":
		if err:=verifyCurrentDB();err!=nil{return fmt.Errorf("migration integrity gate failed: %w",err)}
		report,err:=gate.Evaluate(ctx);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err=enc.Encode(report);err!=nil{return err};if !report.Ready{return errors.New("commercial readiness gate failed")};return nil
	case "fresh-install-db":
		report,err:=runFreshInstallDatabaseCheck(ctx,cfg,versions);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");return enc.Encode(report)
	case "upgrade-db":
		report,err:=runUpgradeDatabaseCheck(ctx,cfg,versions);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");return enc.Encode(report)
	case "record":
		if err:=verifyCurrentDB();err!=nil{return fmt.Errorf("migration integrity gate failed: %w",err)}
		if len(args)<6{return errors.New("usage: readinessctl record <type> <PASS|FAIL> <artifact_ref> <artifact_sha256> <actor> [details_json]")}
		details,err:=readinessDetails(args,6);if err!=nil{return err}
		id,err:=readiness.RecordEvidence(ctx,pool,readiness.EvidenceInput{Type:args[1],Status:args[2],GitCommit:commit,ReleaseVersion:releaseVersion,DatabaseSchema:latestSchema,ArtifactRef:args[3],ArtifactSHA256:args[4],Actor:args[5],Details:details,CompletedAt:time.Now().UTC()});if err!=nil{return err};fmt.Fprintf(os.Stdout,"evidence_id=%d\n",id);return nil
	case "record-file":
		if err:=verifyCurrentDB();err!=nil{return fmt.Errorf("migration integrity gate failed: %w",err)}
		if len(args)<5{return errors.New("usage: readinessctl record-file <type> <PASS|FAIL> <artifact_path> <actor> [details_json]")}
		artifactRef,digest,err:=hashReadinessArtifact(args[3]);if err!=nil{return err}
		details,err:=readinessDetails(args,5);if err!=nil{return err}
		id,err:=readiness.RecordEvidence(ctx,pool,readiness.EvidenceInput{Type:args[1],Status:args[2],GitCommit:commit,ReleaseVersion:releaseVersion,DatabaseSchema:latestSchema,ArtifactRef:artifactRef,ArtifactSHA256:digest,Actor:args[4],Details:details,CompletedAt:time.Now().UTC()});if err!=nil{return err};fmt.Fprintf(os.Stdout,"evidence_id=%d artifact_sha256=%s\n",id,digest);return nil
	default:return fmt.Errorf("readinessctl command %q is not implemented",args[0])
	}
}

func readinessDetails(args []string,index int)(map[string]any,error){details:=map[string]any{};if len(args)>index{if err:=json.Unmarshal([]byte(args[index]),&details);err!=nil{return nil,fmt.Errorf("invalid details_json: %w",err)}};return details,nil}

func hashReadinessArtifact(path string)(string,string,error){
	path=strings.TrimSpace(path);if path==""{return "","",errors.New("artifact path is required")}
	clean:=filepath.Clean(path);info,err:=os.Lstat(clean);if err!=nil{return "","",fmt.Errorf("stat readiness artifact: %w",err)}
	if info.Mode()&os.ModeSymlink!=0{return "","",errors.New("readiness artifact must not be a symlink")}
	if !info.Mode().IsRegular(){return "","",errors.New("readiness artifact must be a regular file")}
	if info.Size()<=0{return "","",errors.New("readiness artifact must not be empty")}
	if info.Size()>maxReadinessArtifactBytes{return "","",fmt.Errorf("readiness artifact exceeds %d bytes",maxReadinessArtifactBytes)}
	if len(clean)>240{return "","",errors.New("readiness artifact path is too long")}
	f,err:=os.Open(clean);if err!=nil{return "","",err};defer f.Close()
	opened,err:=f.Stat();if err!=nil{return "","",err};if !os.SameFile(info,opened){return "","",errors.New("readiness artifact changed before hashing")}
	h:=sha256.New();n,err:=io.Copy(h,io.LimitReader(f,maxReadinessArtifactBytes+1));if err!=nil{return "","",err};if n!=opened.Size(){return "","",errors.New("readiness artifact changed while hashing")};return clean,hex.EncodeToString(h.Sum(nil)),nil
}
