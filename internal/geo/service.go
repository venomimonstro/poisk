package geo

import (
	"context"
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/venomimonstro/poisk/internal/geo/backend"
)

var (
	ErrInvalidQuery = errors.New("invalid geo query")
	keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,95}$`)
)

type Searcher interface { Search(context.Context,backend.Query)(backend.Result,error) }

type Service struct { Backend Searcher; MaxCandidates int }

type Request struct {
	Text string
	CityKey string
	CategoryKey string
	Latitude *float64
	Longitude *float64
	RadiusMeters int
	Limit int
}

type Result struct {
	ID int64 `json:"id"`
	Score float64 `json:"score"`
	Name string `json:"name"`
	Address string `json:"address"`
	CityKey string `json:"city_key,omitempty"`
	CategoryKey string `json:"category_key,omitempty"`
	Phone string `json:"phone,omitempty"`
	Website string `json:"website,omitempty"`
	Latitude *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	DistanceMeters *int `json:"distance_meters,omitempty"`
	QualityScore float64 `json:"quality_score"`
	SourceCount int `json:"source_count"`
}

type Response struct {
	Total int64 `json:"total"`
	TookMS int64 `json:"took_ms"`
	Results []Result `json:"results"`
}

func (s Service) Search(ctx context.Context,req Request)(Response,error){
	if s.Backend==nil{return Response{},errors.New("geo backend is not initialized")}
	req.Text=strings.TrimSpace(req.Text);req.CityKey=strings.ToLower(strings.TrimSpace(req.CityKey));req.CategoryKey=strings.ToLower(strings.TrimSpace(req.CategoryKey))
	if len([]rune(req.Text))>256{return Response{},ErrInvalidQuery}
	if req.CityKey!=""&&!keyPattern.MatchString(req.CityKey){return Response{},ErrInvalidQuery}
	if req.CategoryKey!=""&&!keyPattern.MatchString(req.CategoryKey){return Response{},ErrInvalidQuery}
	if (req.Latitude==nil)!=(req.Longitude==nil){return Response{},ErrInvalidQuery}
	if req.Latitude!=nil{
		if !finite(*req.Latitude)||!finite(*req.Longitude)||*req.Latitude< -90||*req.Latitude>90||*req.Longitude< -180||*req.Longitude>180{return Response{},ErrInvalidQuery}
		if req.RadiusMeters<=0{req.RadiusMeters=5000}
		if req.RadiusMeters>100000{return Response{},ErrInvalidQuery}
	}
	if req.Text==""&&req.CityKey==""&&req.CategoryKey==""&&req.Latitude==nil{return Response{},ErrInvalidQuery}
	if req.Limit<=0{req.Limit=20};if req.Limit>50{req.Limit=50}
	candidates:=s.MaxCandidates;if candidates<=0{candidates=200};if candidates<req.Limit{candidates=req.Limit};if candidates>200{candidates=200}

	backendResult,err:=s.Backend.Search(ctx,backend.Query{Text:req.Text,CityKey:req.CityKey,CategoryKey:req.CategoryKey,Latitude:req.Latitude,Longitude:req.Longitude,RadiusMeters:req.RadiusMeters,Limit:candidates})
	if err!=nil{return Response{},err}
	results:=make([]Result,0,len(backendResult.Hits))
	for _,hit:=range backendResult.Hits{
		result:=Result{ID:hit.ID,Score:hit.Score,Name:hit.Name,Address:hit.Address,CityKey:hit.CityKey,CategoryKey:hit.CategoryKey,Phone:hit.Phone,Website:hit.Website,QualityScore:clamp100(hit.QualityScore),SourceCount:hit.SourceCount}
		if hit.HasLocation{
			lat,lon:=hit.Latitude,hit.Longitude;result.Latitude=&lat;result.Longitude=&lon
			if req.Latitude!=nil{distance:=int(math.Round(haversineMeters(*req.Latitude,*req.Longitude,lat,lon)));result.DistanceMeters=&distance}
		}
		result.Score=rankScore(result.Score,result.QualityScore,result.SourceCount,result.DistanceMeters)
		results=append(results,result)
	}
	sort.SliceStable(results,func(i,j int)bool{
		if results[i].Score==results[j].Score{return results[i].ID<results[j].ID}
		return results[i].Score>results[j].Score
	})
	if len(results)>req.Limit{results=results[:req.Limit]}
	return Response{Total:backendResult.Total,TookMS:backendResult.TookMS,Results:results},nil
}

func rankScore(textScore,quality float64,sources int,distance *int)float64{
	if textScore<0{textScore=0};quality=clamp100(quality)
	sourceBoost:=math.Log1p(float64(maxInt(sources,0)))*2
	distancePenalty:=0.0
	if distance!=nil{distancePenalty=math.Log1p(float64(maxInt(*distance,0))/250.0)*3}
	return textScore+quality*0.05+sourceBoost-distancePenalty
}

func haversineMeters(lat1,lon1,lat2,lon2 float64)float64{
	const earth=6371008.8
	toRad:=math.Pi/180
	p1,p2:=lat1*toRad,lat2*toRad
	dp:=(lat2-lat1)*toRad;dl:=(lon2-lon1)*toRad
	a:=math.Sin(dp/2)*math.Sin(dp/2)+math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return earth*2*math.Atan2(math.Sqrt(a),math.Sqrt(1-a))
}
func clamp100(v float64)float64{if v<0{return 0};if v>100{return 100};return v}
func finite(v float64)bool{return !math.IsNaN(v)&&!math.IsInf(v,0)}
func maxInt(a,b int)int{if a>b{return a};return b}
