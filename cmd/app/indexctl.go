package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/source"
	"github.com/venomimonstro/poisk/internal/platform/config"
)

func runIndexCtl(ctx context.Context,cfg config.Config,pool *pgxpool.Pool,args []string)error{
	if len(args)!=1||args[0]!="rebuild-web"{return errors.New("usage: indexctl rebuild-web")}
	client,err:=indexmanticore.New(indexmanticore.Config{BaseURL:fmt.Sprintf("http://%s:%d",cfg.ManticoreHost,cfg.ManticoreHTTPPort)});if err!=nil{return err}
	if err:=client.ResetWebSchema(ctx);err!=nil{return fmt.Errorf("reset web index: %w",err)}
	repo:=source.NewRepository(pool);var after int64;var applied int
	for{
		docs,err:=repo.RebuildBatch(ctx,after,1000);if err!=nil{return err};if len(docs)==0{break}
		for _,doc:=range docs{
			if doc.NoIndex{after=doc.ID;continue}
			ok,err:=client.Apply(ctx,indexmanticore.Document{ID:doc.ID,EntityVersion:doc.Version,Title:doc.Title,Description:doc.Description,Body:doc.Body,URL:doc.URL,Host:doc.Host,Lang:doc.Lang,ContentHash:doc.ContentHash,QualityScore:doc.QualityScore,SpamScore:doc.SpamScore,AuthorityScore:doc.AuthorityScore,FetchedAtUnix:doc.FetchedAt.Unix()});if err!=nil{return fmt.Errorf("rebuild web document %d: %w",doc.ID,err)};if ok{applied++};after=doc.ID
		}
		if err:=ctx.Err();err!=nil{return err}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"status":"rebuilt","web_documents":applied})
}
