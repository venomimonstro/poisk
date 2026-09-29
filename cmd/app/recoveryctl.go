package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	platformrecovery "github.com/venomimonstro/poisk/internal/platform/recovery"
)

func runRecoveryCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: recoveryctl inspect-artifact <backup-dir> | record <BACKUP|RESTORE> <PASS|FAIL> <artifact-ref|-> <sha256|-> <bytes|-> <duration-ms|-> | record-artifact <BACKUP|RESTORE> <backup-dir> <duration-ms>")}
	repo:=platformrecovery.Repository{DB:pool}
	actor:=strings.TrimSpace(os.Getenv("RECOVERY_ACTOR"));if actor==""{actor="recoveryctl"}
	candidateSchema:=func()(int64,error){var schema int64;if err:=pool.QueryRow(ctx,`SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&schema);err!=nil{return 0,err};if schema<=0{return 0,errors.New("candidate schema is unavailable")};return schema,nil}
	binding:=func()(string,string,error){
		commit,err:=capacityBenchmarkCommit();if err!=nil{return "","",errors.New("READINESS_GIT_SHA is required to bind recovery evidence to the exact release")}
		releaseVersion:=strings.TrimSpace(os.Getenv("RELEASE_VERSION"));if releaseVersion==""||releaseVersion=="dev"{return "","",errors.New("RELEASE_VERSION is required to bind recovery evidence to the staged release candidate")}
		if err:=requireReleaseBuildIdentity(commit,releaseVersion);err!=nil{return "","",err}
		return commit,releaseVersion,nil
	}
	switch args[0]{
	case "inspect-artifact":
		if len(args)!=2{return errors.New("usage: recoveryctl inspect-artifact <backup-dir>")};schema,err:=candidateSchema();if err!=nil{return err};artifact,err:=inspectBackupArtifact(args[1],schema);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(artifact)
	case "record":
		commit,releaseVersion,err:=binding();if err!=nil{return err}
		if len(args)!=7{return errors.New("usage: recoveryctl record <BACKUP|RESTORE> <PASS|FAIL> <artifact-ref|-> <sha256|-> <bytes|-> <duration-ms|->")}
		artifact:=strings.TrimSpace(args[3]);if artifact=="-"{artifact=""};sha:=strings.TrimSpace(args[4]);if sha=="-"{sha=""}
		var bytesPtr,durationPtr *int64
		if args[5]!="-"{v,err:=strconv.ParseInt(args[5],10,64);if err!=nil||v<0{return errors.New("invalid bytes")};bytesPtr=&v}
		if args[6]!="-"{v,err:=strconv.ParseInt(args[6],10,64);if err!=nil||v<0{return errors.New("invalid duration-ms")};durationPtr=&v}
		drill,err:=repo.RecordBound(ctx,args[1],args[2],artifact,sha,bytesPtr,durationPtr,actor,commit,releaseVersion);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(drill)
	case "record-artifact":
		commit,releaseVersion,err:=binding();if err!=nil{return err}
		if len(args)!=4{return errors.New("usage: recoveryctl record-artifact <BACKUP|RESTORE> <backup-dir> <duration-ms>")}
		duration,err:=strconv.ParseInt(args[3],10,64);if err!=nil||duration<=0{return errors.New("duration-ms must be positive")};schema,err:=candidateSchema();if err!=nil{return err}
		artifact,err:=inspectBackupArtifact(args[2],schema);if err!=nil{return err};bytes:=artifact.Bytes
		drill,err:=repo.RecordBound(ctx,args[1],"PASS",artifact.Ref,artifact.SHA256,&bytes,&duration,actor,commit,releaseVersion);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(drill)
	default:return errors.New("unknown recoveryctl command")
	}
}
