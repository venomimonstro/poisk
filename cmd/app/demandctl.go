package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/demand"
)

func runDemandCtl(ctx context.Context,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: demandctl status [limit] | suppress <gap_id> <reason> | reopen <gap_id> <reason>")}
	repo:=demand.NewRepository(pool)
	switch args[0]{
	case "status":
		limit:=50;if len(args)>1{v,err:=strconv.Atoi(args[1]);if err!=nil||v<1||v>500{return errors.New("invalid limit")};limit=v}
		items,err:=repo.OpenGaps(ctx,limit);if err!=nil{return err};enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");return enc.Encode(map[string]any{"gaps":items})
	case "suppress","reopen":
		if len(args)<3{return fmt.Errorf("usage: demandctl %s <gap_id> <reason>",args[0])};id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid gap_id")};reason:=strings.TrimSpace(strings.Join(args[2:]," "));if args[0]=="suppress"{err=repo.Suppress(ctx,id,reason)}else{err=repo.Reopen(ctx,id,reason)};if err!=nil{return err};fmt.Fprintf(os.Stdout,"%s gap %d\n",args[0],id);return nil
	default:return fmt.Errorf("unknown demandctl command %q",args[0])
	}
}
