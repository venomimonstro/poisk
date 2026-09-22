package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/datahub"
)

func runDataHubCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: datahubctl status | build [limit] | suppress <page_id> <reason> | reopen <page_id> <reason> | rollback <page_id> <version> <reason>")};repo:=datahub.NewRepository(pool)
	switch args[0]{
	case "status":out,err:=repo.Status(ctx);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");return enc.Encode(out)
	case "build":limit:=250;if len(args)>1{v,err:=strconv.Atoi(args[1]);if err!=nil||v<1||v>1000{return errors.New("invalid limit")};limit=v};now:=time.Now().UTC();d,err:=repo.RebuildDirectoryBatch(ctx,limit,now);if err!=nil{return err};o,err:=repo.RebuildOrganizationBatch(ctx,limit,now);if err!=nil{return err};w,err:=repo.RebuildWebsiteBatch(ctx,limit,now);if err!=nil{return err};trends,err:=repo.MaterializeTrends(ctx,now,200);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(map[string]any{"directories":d,"organizations":o,"websites":w,"trends":trends})
	case "suppress","reopen":if len(args)<3{return fmt.Errorf("usage: datahubctl %s <page_id> <reason>",args[0])};id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid page_id")};reason:=strings.TrimSpace(strings.Join(args[2:]," "));if args[0]=="suppress"{err=repo.SuppressPage(ctx,id,reason)}else{err=repo.ReopenPage(ctx,id,reason)};if err!=nil{return err};fmt.Fprintf(os.Stdout,"%s page %d\n",args[0],id);return nil
	case "rollback":if len(args)<4{return errors.New("usage: datahubctl rollback <page_id> <version> <reason>")};id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid page_id")};version,err:=strconv.ParseInt(args[2],10,64);if err!=nil||version<=0{return errors.New("invalid version")};reason:=strings.TrimSpace(strings.Join(args[3:]," "));if err:=repo.RollbackPage(ctx,id,version,reason,time.Now().UTC());err!=nil{return err};fmt.Fprintf(os.Stdout,"rollback page %d from version %d\n",id,version);return nil
	default:return fmt.Errorf("unknown datahubctl command %q",args[0])
	}
}
