package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) SearchHost(ctx context.Context,q,host string,limit int)(Result,error){
	if c==nil||c.http==nil{return Result{},errors.New("search client is not initialized")}
	q=strings.TrimSpace(q);host=strings.ToLower(strings.TrimSpace(host));if q==""||host==""{return Result{},errors.New("query and host are required")}
	if limit<=0{limit=10};if limit>c.maxResults{limit=c.maxResults}
	payload:=map[string]any{
		"table":"web_documents",
		"query":map[string]any{"bool":map[string]any{"must":[]any{
			map[string]any{"match":map[string]any{"title,description,body":q}},
			map[string]any{"equals":map[string]any{"host":host}},
		}}},
		"limit":limit,
		"_source":[]string{"title","description","url","host","lang","quality_score","spam_score","authority_score"},
		"highlight":map[string]any{"fields":[]string{"title","description","body"},"before_match":"[[","after_match":"]]","limit":280},
		"options":map[string]any{"ranker":"bm25","field_weights":map[string]int{"title":12,"description":4,"body":1},"max_matches":500},
	}
	body,err:=json.Marshal(payload);if err!=nil{return Result{},err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,c.baseURL+"/search",bytes.NewReader(body));if err!=nil{return Result{},err};req.Header.Set("Content-Type","application/json")
	resp,err:=c.http.Do(req);if err!=nil{return Result{},err};defer resp.Body.Close()
	if resp.ContentLength>c.maxBody&&resp.ContentLength>=0{return Result{},ErrResponseTooLarge};raw,err:=io.ReadAll(io.LimitReader(resp.Body,c.maxBody+1));if err!=nil{return Result{},err};if int64(len(raw))>c.maxBody{return Result{},ErrResponseTooLarge};if resp.StatusCode<200||resp.StatusCode>=300{return Result{},fmt.Errorf("%w: status=%d",ErrSearchBackend,resp.StatusCode)}
	var decoded struct{Took int64 `json:"took"`;TimedOut bool `json:"timed_out"`;Hits struct{Total int64 `json:"total"`;Hits []struct{ID int64 `json:"_id"`;Score float64 `json:"_score"`;Source struct{Title string `json:"title"`;Description string `json:"description"`;URL string `json:"url"`;Host string `json:"host"`;Lang string `json:"lang"`;QualityScore float64 `json:"quality_score"`;SpamScore float64 `json:"spam_score"`;AuthorityScore float64 `json:"authority_score"`} `json:"_source"`;Highlight map[string][]string `json:"highlight"`} `json:"hits"`} `json:"hits"`}
	if err:=json.Unmarshal(raw,&decoded);err!=nil{return Result{},fmt.Errorf("decode host search response: %w",err)};if decoded.TimedOut{return Result{},fmt.Errorf("%w: backend timeout",ErrSearchBackend)}
	out:=Result{Total:decoded.Hits.Total,TookMS:decoded.Took,Hits:make([]Hit,0,len(decoded.Hits.Hits))}
	for _,h:=range decoded.Hits.Hits{
		if !strings.EqualFold(h.Source.Host,host){continue}
		out.Hits=append(out.Hits,Hit{ID:h.ID,Score:h.Score,Title:h.Source.Title,Description:h.Source.Description,URL:h.Source.URL,Host:h.Source.Host,Lang:h.Source.Lang,Snippet:firstSnippet(h.Highlight,h.Source.Description),QualityScore:clampScore(h.Source.QualityScore),SpamScore:clampScore(h.Source.SpamScore),AuthorityScore:clampScore(h.Source.AuthorityScore)})
	}
	return out,nil
}
