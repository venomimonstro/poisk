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
	if len(args)==0{return errors.New("usage: recoveryctl record <BACKUP|RESTORE> <PASS|FAIL> <artifact-ref|-> <sha256|-> <bytes|-> <duration-ms|->")}
	repo:=platformrecovery.Repository{DB:pool}
	switch args[0]{
	case "record":
		if len(args)!=7{return errors.New("usage: recoveryctl record <BACKUP|RESTORE> <PASS|FAIL> <artifact-ref|-> <sha256|-> <bytes|-> <duration-ms|->")}
		artifact:=strings.TrimSpace(args[3]);if artifact=="-"{artifact=""};sha:=strings.TrimSpace(args[4]);if sha=="-"{sha=""}
		var bytesPtr,durationPtr *int64
		if args[5]!="-"{v,err:=strconv.ParseInt(args[5],10,64);if err!=nil||v<0{return errors.New("invalid bytes")};bytesPtr=&v}
		if args[6]!="-"{v,err:=strconv.ParseInt(args[6],10,64);if err!=nil||v<0{return errors.New("invalid duration-ms")};durationPtr=&v}
		actor:=strings.TrimSpace(os.Getenv("RECOVERY_ACTOR"));if actor==""{actor="recoveryctl"}
		commit,err:=capacityBenchmarkCommit();if err!=nil{return errors.New("READINESS_GIT_SHA is required to bind recovery evidence to the exact release")}
		drill,err:=repo.RecordBound(ctx,args[1],args[2],artifact,sha,bytesPtr,durationPtr,actor,commit);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(drill)
	default:return errors.New("unknown recoveryctl command")
	}
}
