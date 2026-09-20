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
	"github.com/venomimonstro/poisk/internal/address"
	"github.com/venomimonstro/poisk/internal/address/gar"
	addressindex "github.com/venomimonstro/poisk/internal/address/indexer"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/platform/config"
)

func addressImportRoot()string{if value:=strings.TrimSpace(os.Getenv("ADDRESS_IMPORT_ROOT"));value!=""{return value};return "/address-imports"}

func runAddressCtl(ctx context.Context,cfg config.Config,pool *pgxpool.Pool,args []string)error{
	if len(args)==0{return errors.New("usage: addressctl stage|finish|resolve|status|rebuild-index")}
	repo:=address.NewRepository(pool)
	switch args[0]{
	case "stage":
		if len(args)!=5{return errors.New("usage: addressctl stage <revision> <region-1-99> <addr_obj|house|hierarchy> <relative-xml-path>")}
		region,err:=strconv.Atoi(args[2]);if err!=nil||region<1||region>99{return errors.New("invalid region")}
		kind:=gar.Kind(strings.ToUpper(strings.TrimSpace(args[3])));switch kind{case gar.KindAddress,gar.KindHouse,gar.KindHierarchy:default:return errors.New("invalid GAR kind")}
		path,err:=resolveImportPath(addressImportRoot(),args[4]);if err!=nil{return err}
		batch,err:=repo.CreateBatch(ctx,args[1]);if err!=nil{return err};if batch.Status!="STAGING"{return address.ErrBatchConflict}
		file,err:=os.Open(path);if err!=nil{return err};defer file.Close()
		if err:=(address.Importer{Store:repo,CheckpointEvery:1000}).ImportFile(ctx,batch.ID,address.FileSpec{Name:args[4],RegionCode:region,Kind:kind},file);err!=nil{return err}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"batch_id":batch.ID,"file":args[4]})
	case "finish":
		if len(args)!=2{return errors.New("usage: addressctl finish <batch-id>")};id,err:=strconv.ParseInt(args[1],10,64);if err!=nil||id<=0{return errors.New("invalid batch id")};return repo.FinishStaging(ctx,id)
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
