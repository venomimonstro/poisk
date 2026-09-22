package main

import (
	"context"
	"fmt"

	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
)

func capacityManticoreDocumentCount(ctx context.Context,baseURL string)(int64,error){
	client,err:=indexmanticore.New(indexmanticore.Config{BaseURL:baseURL});if err!=nil{return 0,fmt.Errorf("create capacity Manticore probe: %w",err)}
	count,err:=client.CountDocuments(ctx);if err!=nil{return 0,fmt.Errorf("count Manticore web documents: %w",err)};return count,nil
}
