package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	indexmanticore "github.com/venomimonstro/poisk/internal/indexer/manticore"
)

var (
	ErrAddressNotFound = errors.New("address not found")
	ErrAddressVersionMismatch = errors.New("address version mismatch")
)

type Source struct{ db *pgxpool.Pool }
func NewSource(db *pgxpool.Pool)*Source{return &Source{db:db}}

func (s *Source) Load(ctx context.Context,id,version int64)(indexmanticore.AddressDocument,error){
	if s==nil||s.db==nil{return indexmanticore.AddressDocument{},errors.New("address index source is not initialized")}
	var doc indexmanticore.AddressDocument
	var parent *int64;var lat,lon *float64;var level *int
	err:=s.db.QueryRow(ctx,`SELECT address_id,version,display_name,full_address,normalized_name,region_code,object_kind,level,parent_address_id,latitude,longitude,status FROM addresses WHERE address_id=$1`,id).
		Scan(&doc.ID,&doc.EntityVersion,&doc.DisplayName,&doc.FullAddress,&doc.NormalizedName,&doc.RegionCode,&doc.ObjectKind,&level,&parent,&lat,&lon,&doc.Status)
	if errors.Is(err,pgx.ErrNoRows){return indexmanticore.AddressDocument{},ErrAddressNotFound}
	if err!=nil{return indexmanticore.AddressDocument{},fmt.Errorf("load address: %w",err)}
	if doc.EntityVersion!=version{return indexmanticore.AddressDocument{},fmt.Errorf("%w: requested=%d current=%d",ErrAddressVersionMismatch,version,doc.EntityVersion)}
	if level!=nil{doc.Level=*level};if parent!=nil{doc.ParentAddressID=*parent};if lat!=nil&&lon!=nil{doc.Latitude=*lat;doc.Longitude=*lon;doc.HasLocation=true}
	return doc,nil
}

func (s *Source) RebuildBatch(ctx context.Context,afterID int64,limit int)([]indexmanticore.AddressDocument,error){
	if limit<=0||limit>5000{return nil,errors.New("rebuild limit must be between 1 and 5000")}
	rows,err:=s.db.Query(ctx,`SELECT address_id,version,display_name,full_address,normalized_name,region_code,object_kind,level,parent_address_id,latitude,longitude,status FROM addresses WHERE address_id>$1 AND status='ACTIVE' ORDER BY address_id LIMIT $2`,afterID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	out:=make([]indexmanticore.AddressDocument,0,limit)
	for rows.Next(){var doc indexmanticore.AddressDocument;var parent *int64;var lat,lon *float64;var level *int;if err:=rows.Scan(&doc.ID,&doc.EntityVersion,&doc.DisplayName,&doc.FullAddress,&doc.NormalizedName,&doc.RegionCode,&doc.ObjectKind,&level,&parent,&lat,&lon,&doc.Status);err!=nil{return nil,err};if level!=nil{doc.Level=*level};if parent!=nil{doc.ParentAddressID=*parent};if lat!=nil&&lon!=nil{doc.Latitude=*lat;doc.Longitude=*lon;doc.HasLocation=true};out=append(out,doc)}
	return out,rows.Err()
}
