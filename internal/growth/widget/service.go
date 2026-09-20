package widget

import (
	"context"
	"errors"
	"sort"

	querynorm "github.com/venomimonstro/poisk/internal/query"
	"github.com/venomimonstro/poisk/internal/search/backend"
)

type SearchBackend interface{SearchHost(context.Context,string,string,int)(backend.Result,error)}
type UsageRecorder interface{RecordUsage(context.Context,int64,int)error}

type Service struct{Backend SearchBackend;Usage UsageRecorder}
type SearchRequest struct{Query string `json:"q"`;Limit int `json:"limit,omitempty"`}
type SearchResult struct{ID int64 `json:"id"`;Title string `json:"title"`;URL string `json:"url"`;Snippet string `json:"snippet"`;Score float64 `json:"score"`}
type SearchResponse struct{Query string `json:"query"`;Normalized string `json:"normalized"`;Total int64 `json:"total"`;TookMS int64 `json:"took_ms"`;Results []SearchResult `json:"results"`}

func (s Service) Search(ctx context.Context,cfg Config,req SearchRequest)(SearchResponse,error){
	if s.Backend==nil{return SearchResponse{},errors.New("widget search backend is not initialized")}
	norm,err:=querynorm.Normalize(req.Query);if err!=nil{return SearchResponse{},err}
	limit:=req.Limit;if limit<=0{limit=10};if cfg.MaxResults>0&&limit>cfg.MaxResults{limit=cfg.MaxResults};if limit>20{limit=20}
	queries:=append([]string{norm.Primary},norm.Variants...);var found backend.Result
	for _,q:=range queries{found,err=s.Backend.SearchHost(ctx,q,cfg.Host,minInt(limit*2,40));if err!=nil{return SearchResponse{},err};if len(found.Hits)>0{break}}
	hits:=append([]backend.Hit(nil),found.Hits...)
	for i:=range hits{factor:=1+0.002*hits[i].QualityScore+0.001*hits[i].AuthorityScore-0.008*hits[i].SpamScore;if factor<0.20{factor=0.20};if factor>1.30{factor=1.30};hits[i].Score*=factor}
	sort.SliceStable(hits,func(i,j int)bool{return hits[i].Score>hits[j].Score})
	out:=SearchResponse{Query:req.Query,Normalized:norm.Primary,Total:found.Total,TookMS:found.TookMS,Results:make([]SearchResult,0,limit)}
	for _,h:=range hits{if len(out.Results)>=limit{break};out.Results=append(out.Results,SearchResult{ID:h.ID,Title:h.Title,URL:h.URL,Snippet:h.Snippet,Score:h.Score})}
	if s.Usage!=nil{_ = s.Usage.RecordUsage(ctx,cfg.WidgetID,len(out.Results))}
	return out,nil
}
func minInt(a,b int)int{if a<b{return a};return b}
