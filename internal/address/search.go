package address

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	addressbackend "github.com/venomimonstro/poisk/internal/address/backend"
)

var ErrInvalidQuery=errors.New("invalid address query")

type SearchBackend interface{Search(context.Context,addressbackend.Query)(addressbackend.Result,error)}
type SearchService struct{Backend SearchBackend;DB *pgxpool.Pool}

type SearchRequest struct{Text string;RegionCode int;Limit int;Prefix bool}
type GeocodeResult struct{AddressID int64 `json:"address_id"`;FullAddress string `json:"full_address"`;Latitude *float64 `json:"latitude,omitempty"`;Longitude *float64 `json:"longitude,omitempty"`;HasLocation bool `json:"has_location"`;Confidence float64 `json:"confidence"`}
type ReverseResult struct{AddressID int64 `json:"address_id"`;FullAddress string `json:"full_address"`;Latitude float64 `json:"latitude"`;Longitude float64 `json:"longitude"`;DistanceMeters float64 `json:"distance_meters"`}

func (s SearchService) Search(ctx context.Context,req SearchRequest)(addressbackend.Result,error){
	if s.Backend==nil{return addressbackend.Result{},errors.New("address backend is unavailable")};req.Text=strings.TrimSpace(req.Text);if req.Text==""||len([]rune(req.Text))>256||req.RegionCode<0||req.RegionCode>99{return addressbackend.Result{},ErrInvalidQuery};if req.Limit<=0{req.Limit=10};if req.Limit>50{req.Limit=50};return s.Backend.Search(ctx,addressbackend.Query{Text:req.Text,RegionCode:req.RegionCode,Limit:req.Limit,Prefix:req.Prefix})
}

func (s SearchService) Forward(ctx context.Context,addressID int64)(GeocodeResult,error){
	if s.DB==nil||addressID<=0{return GeocodeResult{},ErrInvalidQuery};var out GeocodeResult;var lat,lon *float64;err:=s.DB.QueryRow(ctx,`SELECT address_id,full_address,latitude,longitude FROM addresses WHERE address_id=$1 AND status='ACTIVE'`,addressID).Scan(&out.AddressID,&out.FullAddress,&lat,&lon);if err!=nil{return GeocodeResult{},err};out.Latitude=lat;out.Longitude=lon;out.HasLocation=lat!=nil&&lon!=nil;if out.HasLocation{out.Confidence=1}else{out.Confidence=0};return out,nil
}

func (s SearchService) Reverse(ctx context.Context,lat,lon float64,radiusMeters,limit int)([]ReverseResult,error){
	if s.DB==nil||math.IsNaN(lat)||math.IsNaN(lon)||math.IsInf(lat,0)||math.IsInf(lon,0)||lat < -90||lat > 90||lon < -180||lon > 180{return nil,ErrInvalidQuery};if radiusMeters<=0{radiusMeters=500};if radiusMeters>5000{return nil,ErrInvalidQuery};if limit<=0{limit=5};if limit>20{limit=20}
	rows,err:=s.DB.Query(ctx,`SELECT address_id,full_address,latitude,longitude,ST_Distance(location,ST_SetSRID(ST_MakePoint($2,$1),4326)::geography) AS distance_m
FROM addresses WHERE status='ACTIVE' AND location IS NOT NULL AND ST_DWithin(location,ST_SetSRID(ST_MakePoint($2,$1),4326)::geography,$3)
ORDER BY distance_m ASC,address_id ASC LIMIT $4`,lat,lon,radiusMeters,limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]ReverseResult,0,limit);for rows.Next(){var item ReverseResult;if err:=rows.Scan(&item.AddressID,&item.FullAddress,&item.Latitude,&item.Longitude,&item.DistanceMeters);err!=nil{return nil,err};out=append(out,item)};return out,rows.Err()
}
