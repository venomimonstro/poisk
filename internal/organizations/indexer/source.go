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
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrOrganizationVersionMismatch = errors.New("organization version mismatch")
)

type Source struct{ db *pgxpool.Pool }
func NewSource(db *pgxpool.Pool)*Source{return &Source{db:db}}

func (s *Source) Load(ctx context.Context,placeID,version int64)(indexmanticore.OrganizationDocument,error){
	if s==nil||s.db==nil{return indexmanticore.OrganizationDocument{},errors.New("organization index source is not initialized")}
	var doc indexmanticore.OrganizationDocument
	var latitude,longitude *float64
	err:=s.db.QueryRow(ctx,`SELECT place_id,version,name,COALESCE(address,''),COALESCE(city_key,''),COALESCE(category_key,''),COALESCE(phone,''),COALESCE(website,''),
normalized_name,COALESCE(normalized_address,''),latitude,longitude,quality_score,source_count,status
FROM organizations WHERE place_id=$1`,placeID).Scan(&doc.ID,&doc.EntityVersion,&doc.Name,&doc.Address,&doc.CityKey,&doc.CategoryKey,&doc.Phone,&doc.Website,&doc.NormalizedName,&doc.NormalizedAddress,&latitude,&longitude,&doc.QualityScore,&doc.SourceCount,&doc.Status)
	if errors.Is(err,pgx.ErrNoRows){return indexmanticore.OrganizationDocument{},ErrOrganizationNotFound}
	if err!=nil{return indexmanticore.OrganizationDocument{},fmt.Errorf("load organization for index: %w",err)}
	if doc.EntityVersion!=version{return indexmanticore.OrganizationDocument{},fmt.Errorf("%w: requested=%d current=%d",ErrOrganizationVersionMismatch,version,doc.EntityVersion)}
	if latitude!=nil&&longitude!=nil{doc.Latitude=*latitude;doc.Longitude=*longitude;doc.HasLocation=true}
	return doc,nil
}

func (s *Source) RebuildBatch(ctx context.Context,afterPlaceID int64,limit int)([]indexmanticore.OrganizationDocument,error){
	if s==nil||s.db==nil{return nil,errors.New("organization index source is not initialized")}
	if limit<=0||limit>5000{return nil,errors.New("rebuild limit must be between 1 and 5000")}
	rows,err:=s.db.Query(ctx,`SELECT place_id,version,name,COALESCE(address,''),COALESCE(city_key,''),COALESCE(category_key,''),COALESCE(phone,''),COALESCE(website,''),
normalized_name,COALESCE(normalized_address,''),latitude,longitude,quality_score,source_count,status
FROM organizations WHERE place_id>$1 AND status IN ('ACTIVE','REVIEW') ORDER BY place_id LIMIT $2`,afterPlaceID,limit)
	if err!=nil{return nil,err};defer rows.Close()
	out:=make([]indexmanticore.OrganizationDocument,0,limit)
	for rows.Next(){
		var doc indexmanticore.OrganizationDocument;var lat,lon *float64
		if err:=rows.Scan(&doc.ID,&doc.EntityVersion,&doc.Name,&doc.Address,&doc.CityKey,&doc.CategoryKey,&doc.Phone,&doc.Website,&doc.NormalizedName,&doc.NormalizedAddress,&lat,&lon,&doc.QualityScore,&doc.SourceCount,&doc.Status);err!=nil{return nil,err}
		if lat!=nil&&lon!=nil{doc.Latitude=*lat;doc.Longitude=*lon;doc.HasLocation=true}
		out=append(out,doc)
	}
	return out,rows.Err()
}
