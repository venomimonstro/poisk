package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrResponseTooLarge = errors.New("geo response exceeds hard limit")
	ErrBackend = errors.New("geo backend error")
)

type Config struct {
	BaseURL string
	Timeout time.Duration
	MaxResponseBytes int64
	MaxResults int
}

type Client struct {
	baseURL string
	maxBody int64
	maxResults int
	http *http.Client
}

type Query struct {
	Text string
	CityKey string
	CategoryKey string
	Latitude *float64
	Longitude *float64
	RadiusMeters int
	Limit int
}

type Hit struct {
	ID int64 `json:"id"`
	Score float64 `json:"score"`
	Name string `json:"name"`
	Address string `json:"address"`
	CityKey string `json:"city_key"`
	CategoryKey string `json:"category_key"`
	Phone string `json:"phone,omitempty"`
	Website string `json:"website,omitempty"`
	Latitude float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	HasLocation bool `json:"has_location"`
	QualityScore float64 `json:"quality_score"`
	SourceCount int `json:"source_count"`
}

type Result struct { Total int64; TookMS int64; Hits []Hit }

func New(cfg Config)(*Client,error){
	if cfg.BaseURL==""{cfg.BaseURL="http://manticore:9308"}
	if cfg.Timeout<=0{cfg.Timeout=1200*time.Millisecond}
	if cfg.MaxResponseBytes<=0{cfg.MaxResponseBytes=2<<20}
	if cfg.MaxResults<=0{cfg.MaxResults=200}
	u,err:=url.Parse(strings.TrimRight(cfg.BaseURL,"/"));if err!=nil||(u.Scheme!="http"&&u.Scheme!="https")||u.Host==""{return nil,errors.New("invalid manticore base URL")}
	h:=&http.Client{Timeout:cfg.Timeout};h.CheckRedirect=func(_ *http.Request,_ []*http.Request)error{return http.ErrUseLastResponse}
	return &Client{baseURL:u.String(),maxBody:cfg.MaxResponseBytes,maxResults:cfg.MaxResults,http:h},nil
}

func (c *Client) Search(ctx context.Context,q Query)(Result,error){
	if c==nil||c.http==nil{return Result{},errors.New("geo client is not initialized")}
	limit:=q.Limit;if limit<=0{limit=20};if limit>c.maxResults{limit=c.maxResults}
	must:=[]any{map[string]any{"equals":map[string]any{"status":"ACTIVE"}}}
	if text:=strings.TrimSpace(q.Text);text!=""{must=append(must,map[string]any{"match":map[string]any{"name,address":text}})}
	if city:=strings.TrimSpace(q.CityKey);city!=""{must=append(must,map[string]any{"equals":map[string]any{"city_key":city}})}
	if category:=strings.TrimSpace(q.CategoryKey);category!=""{must=append(must,map[string]any{"equals":map[string]any{"category_key":category}})}
	if q.Latitude!=nil&&q.Longitude!=nil{
		radius:=q.RadiusMeters;if radius<=0{radius=5000};if radius>100000{radius=100000}
		must=append(must,map[string]any{"equals":map[string]any{"has_location":1}},map[string]any{"geo_distance":map[string]any{
			"distance_type":"adaptive","location_anchor":map[string]any{"lat":*q.Latitude,"lon":*q.Longitude},"location_source":"latitude,longitude","distance":fmt.Sprintf("%d m",radius),
		}})
	}
	payload:=map[string]any{
		"table":"organizations","query":map[string]any{"bool":map[string]any{"must":must}},"limit":limit,
		"_source":[]string{"name","address","city_key","category_key","phone","website","latitude","longitude","has_location","quality_score","source_count"},
		"options":map[string]any{"ranker":"bm25","field_weights":map[string]int{"name":12,"address":3},"max_matches":1000},
	}
	body,err:=json.Marshal(payload);if err!=nil{return Result{},err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,c.baseURL+"/search",bytes.NewReader(body));if err!=nil{return Result{},err}
	req.Header.Set("Content-Type","application/json")
	resp,err:=c.http.Do(req);if err!=nil{return Result{},err};defer resp.Body.Close()
	if resp.ContentLength>c.maxBody&&resp.ContentLength>=0{return Result{},ErrResponseTooLarge}
	raw,err:=io.ReadAll(io.LimitReader(resp.Body,c.maxBody+1));if err!=nil{return Result{},err};if int64(len(raw))>c.maxBody{return Result{},ErrResponseTooLarge}
	if resp.StatusCode<200||resp.StatusCode>=300{return Result{},fmt.Errorf("%w: status=%d",ErrBackend,resp.StatusCode)}
	var decoded struct{
		Took int64 `json:"took"`;TimedOut bool `json:"timed_out"`;Hits struct{Total int64 `json:"total"`;Hits []struct{ID int64 `json:"_id"`;Score float64 `json:"_score"`;Source Hit `json:"_source"`} `json:"hits"`} `json:"hits"`
	}
	if err:=json.Unmarshal(raw,&decoded);err!=nil{return Result{},fmt.Errorf("decode geo response: %w",err)}
	if decoded.TimedOut{return Result{},fmt.Errorf("%w: backend timeout",ErrBackend)}
	out:=Result{Total:decoded.Hits.Total,TookMS:decoded.Took,Hits:make([]Hit,0,len(decoded.Hits.Hits))}
	for _,item:=range decoded.Hits.Hits{h:=item.Source;h.ID=item.ID;h.Score=item.Score;out.Hits=append(out.Hits,h)}
	return out,nil
}
