package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/address"
	addressindex "github.com/venomimonstro/poisk/internal/address/indexer"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/platform/config"
)

func runAddressCtl(ctx context.Context,cfg config.Config,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: addressctl resolve|status|rebuild-index")}
	repo:=address.NewRepository(pool)
	switch args[0]{
	case "resolve":
		if len(args)!=2{return errors.New("usage: addressctl resolve <batch-id>")}
		id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid batch id")}
		result,err:=repo.ResolveBatch(ctx,id);if err!=nil{return err}
		return json.NewEncoder(os.Stdout).Encode(result)
	case "status":
		if len(args)!=2{return errors.New("usage: addressctl status <batch-id>")}
		id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid batch id")}
		batch,err:=repo.Batch(ctx,id);if err!=nil{return err};return json.NewEncoder(os.Stdout).Encode(batch)
	case "rebuild-index":
		client,err:=indexmanticore.New(indexmanticore.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort)});if err!=nil{return err}
		if err:=client.ResetAddressesSchema(ctx);err!=nil{return err}
		source:=addressindex.NewSource(pool)
		after:=int64(0)
		for{docs,err:=source.RebuildBatch(ctx,after,1000);if err!=nil{return err};if len(docs)==0{break};for _,doc:=range docs{if _,err:=client.ApplyAddress(ctx,doc);err!=nil{return err};after=doc.ID}}
		return nil
	default:return fmt.Errorf("unknown addressctl command %q",args[0])
	}
}
