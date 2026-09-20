package geo

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/venomimonstro/poisk/internal/geo/backend"
)

type ViewportRequest struct {
	MinLatitude float64
	MinLongitude float64
	MaxLatitude float64
	MaxLongitude float64
	Zoom float64
	CityKey string
	CategoryKey string
	Limit int
}

type Cluster struct {
	ID string `json:"id"`
	Latitude float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Count int `json:"count"`
	PlaceIDs []int64 `json:"place_ids,omitempty"`
}

type ViewportResponse struct {
	Total int64 `json:"total"`
	TookMS int64 `json:"took_ms"`
	Results []Result `json:"results"`
	Clusters []Cluster `json:"clusters"`
}

func (s Service) Viewport(ctx context.Context,req ViewportRequest)(ViewportResponse,error){
	if s.Backend==nil{return ViewportResponse{},fmt.Errorf("geo backend is not initialized")}
	if !finite(req.MinLatitude)||!finite(req.MinLongitude)||!finite(req.MaxLatitude)||!finite(req.MaxLongitude)||
		req.MinLatitude < -85 || req.MaxLatitude > 85 || req.MinLongitude < -180 || req.MaxLongitude > 180 ||
		req.MinLatitude >= req.MaxLatitude || req.MinLongitude >= req.MaxLongitude{return ViewportResponse{},ErrInvalidQuery}
	if !finite(req.Zoom)||req.Zoom<0||req.Zoom>24{return ViewportResponse{},ErrInvalidQuery}
	req.CityKey=strings.ToLower(strings.TrimSpace(req.CityKey));req.CategoryKey=strings.ToLower(strings.TrimSpace(req.CategoryKey))
	if req.CityKey!=""&&!keyPattern.MatchString(req.CityKey){return ViewportResponse{},ErrInvalidQuery}
	if req.CategoryKey!=""&&!keyPattern.MatchString(req.CategoryKey){return ViewportResponse{},ErrInvalidQuery}
	if req.Limit<=0{req.Limit=200};if req.Limit>200{req.Limit=200}

	minLat,minLon,maxLat,maxLon:=req.MinLatitude,req.MinLongitude,req.MaxLatitude,req.MaxLongitude
	backendResult,err:=s.Backend.Search(ctx,backend.Query{CityKey:req.CityKey,CategoryKey:req.CategoryKey,MinLatitude:&minLat,MinLongitude:&minLon,MaxLatitude:&maxLat,MaxLongitude:&maxLon,Limit:req.Limit})
	if err!=nil{return ViewportResponse{},err}

	results:=make([]Result,0,len(backendResult.Hits))
	for _,hit:=range backendResult.Hits{
		if !hit.HasLocation{continue}
		lat,lon:=hit.Latitude,hit.Longitude
		results=append(results,Result{ID:hit.ID,Score:rankScore(hit.Score,hit.QualityScore,hit.SourceCount,nil),Name:hit.Name,Address:hit.Address,CityKey:hit.CityKey,CategoryKey:hit.CategoryKey,Phone:hit.Phone,Website:hit.Website,Latitude:&lat,Longitude:&lon,QualityScore:clamp100(hit.QualityScore),SourceCount:hit.SourceCount})
	}
	sort.SliceStable(results,func(i,j int)bool{if results[i].Score==results[j].Score{return results[i].ID<results[j].ID};return results[i].Score>results[j].Score})
	clusters:=clusterViewport(results,req)
	return ViewportResponse{Total:backendResult.Total,TookMS:backendResult.TookMS,Results:results,Clusters:clusters},nil
}

func clusterViewport(results []Result,req ViewportRequest)[]Cluster{
	if len(results)==0{return []Cluster{}}
	grid:=8
	if req.Zoom>=8{grid=16};if req.Zoom>=12{grid=32};if req.Zoom>=16{grid=64}
	latStep:=(req.MaxLatitude-req.MinLatitude)/float64(grid)
	lonStep:=(req.MaxLongitude-req.MinLongitude)/float64(grid)
	if latStep<=0||lonStep<=0{return []Cluster{}}
	type acc struct{lat,lon float64;count int;ids []int64}
	cells:=map[string]*acc{}
	for _,result:=range results{
		if result.Latitude==nil||result.Longitude==nil{continue}
		x:=int(math.Floor((*result.Longitude-req.MinLongitude)/lonStep));y:=int(math.Floor((*result.Latitude-req.MinLatitude)/latStep))
		if x<0{x=0};if y<0{y=0};if x>=grid{x=grid-1};if y>=grid{y=grid-1}
		key:=fmt.Sprintf("%d:%d",x,y)
		item:=cells[key];if item==nil{item=&acc{};cells[key]=item}
		item.lat+=*result.Latitude;item.lon+=*result.Longitude;item.count++
		if len(item.ids)<20{item.ids=append(item.ids,result.ID)}
	}
	keys:=make([]string,0,len(cells));for key:=range cells{keys=append(keys,key)};sort.Strings(keys)
	clusters:=make([]Cluster,0,len(keys))
	for _,key:=range keys{item:=cells[key];clusters=append(clusters,Cluster{ID:key,Latitude:item.lat/float64(item.count),Longitude:item.lon/float64(item.count),Count:item.count,PlaceIDs:item.ids})}
	return clusters
}
