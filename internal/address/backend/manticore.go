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

var ErrBackend=errors.New("address search backend error")

type Config struct{BaseURL string;Timeout time.Duration;MaxResponseBytes int64;MaxResults int}
type Client struct{baseURL string;maxBody int64;maxResults int;http *http.Client}
type Query struct{Text string;RegionCode int;Limit int;Prefix bool}
type Hit struct{ID int64 `json:"address_id"`;Score float64 `json:"score"`;DisplayName string `json:"display_name"`;FullAddress string `json:"full_address"`;RegionCode int `json:"region_code"`;ObjectKind string `json:"object_kind"`;Level int `json:"level"`;ParentAddressID int64 `json:"parent_address_id,omitempty"`;Latitude float64 `json:"latitude,omitempty"`;Longitude float64 `json:"longitude,omitempty"`;HasLocation bool `json:"has_location"`}
type Result struct{Total int64 `json:"total"`;TookMS int64 `json:"took_ms"`;Hits []Hit `json:"results"`}

func New(cfg Config)(*Client,error){if cfg.BaseURL==""{cfg.BaseURL="http://manticore:9308"};if cfg.Timeout<=0{cfg.Timeout=900*time.Millisecond};if cfg.MaxResponseBytes<=0{cfg.MaxResponseBytes=1<<20};if cfg.MaxResults<=0{cfg.MaxResults=50};u,err:=url.Parse(strings.TrimRight(cfg.BaseURL,"/"));if err!=nil||(u.Scheme!="http"&&u.Scheme!="https")||u.Host==""{return nil,errors.New("invalid address backend URL")};h:=&http.Client{Timeout:cfg.Timeout};h.CheckRedirect=func(_ *http.Request,_ []*http.Request)error{return http.ErrUseLastResponse};return &Client{baseURL:u.String(),maxBody:cfg.MaxResponseBytes,maxResults:cfg.MaxResults,http:h},nil}

func (c *Client) Search(ctx context.Context,q Query)(Result,error){
	text:=strings.TrimSpace(q.Text);if text==""||len([]rune(text))>256{return Result{},errors.New("invalid address query")};limit:=q.Limit;if limit<=0{limit=10};if limit>c.maxResults{limit=c.maxResults}
	must:=[]any{map[string]any{"equals":map[string]any{"status":"ACTIVE"}}}
	matchText:=text;if q.Prefix{matchText=text+"*"}
	must=append(must,map[string]any{"match":map[string]any{"display_name,full_address":matchText}})
	if q.RegionCode>0{must=append(must,map[string]any{"equals":map[string]any{"region_code":q.RegionCode}})}
	payload:=map[string]any{"table":"addresses","query":map[string]any{"bool":map[string]any{"must":must}},"limit":limit,"_source":[]string{"display_name","full_address","region_code","object_kind","level","parent_address_id","latitude","longitude","has_location"},"options":map[string]any{"ranker":"bm25","field_weights":map[string]int{"display_name":8,"full_address":3},"max_matches":500}}
	body,err:=json.Marshal(payload);if err!=nil{return Result{},err};req,err:=http.NewRequestWithContext(ctx,http.MethodPost,c.baseURL+"/search",bytes.NewReader(body));if err!=nil{return Result{},err};req.Header.Set("Content-Type","application/json")
	resp,err:=c.http.Do(req);if err!=nil{return Result{},err};defer resp.Body.Close();raw,err:=io.ReadAll(io.LimitReader(resp.Body,c.maxBody+1));if err!=nil{return Result{},err};if int64(len(raw))>c.maxBody{return Result{},ErrBackend};if resp.StatusCode<200||resp.StatusCode>=300{return Result{},fmt.Errorf("%w: status=%d",ErrBackend,resp.StatusCode)}
	var decoded struct{Took int64 `json:"took"`;TimedOut bool `json:"timed_out"`;Hits struct{Total int64 `json:"total"`;Hits []struct{ID int64 `json:"_id"`;Score float64 `json:"_score"`;Source Hit `json:"_source"`} `json:"hits"`} `json:"hits"`};if err:=json.Unmarshal(raw,&decoded);err!=nil{return Result{},fmt.Errorf("decode address response: %w",err)};if decoded.TimedOut{return Result{},ErrBackend};out:=Result{Total:decoded.Hits.Total,TookMS:decoded.Took,Hits:make([]Hit,0,len(decoded.Hits.Hits))};for _,item:=range decoded.Hits.Hits{h:=item.Source;h.ID=item.ID;h.Score=item.Score;out.Hits=append(out.Hits,h)};return out,nil
}
