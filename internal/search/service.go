package search

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	querynorm "github.com/venomimonstro/poisk/internal/query"
	"github.com/venomimonstro/poisk/internal/search/backend"
)

type Backend interface {
	Search(ctx context.Context, q string, limit int) (backend.Result, error)
}

type Service struct {
	Backend Backend
	Cache   *Cache
}

type Request struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type Response struct {
	Query      string   `json:"query"`
	Normalized string   `json:"normalized"`
	UsedQuery  string   `json:"used_query"`
	Total      int64    `json:"total"`
	TookMS     int64    `json:"took_ms"`
	Results    []Result `json:"results"`
	Cached     bool     `json:"cached"`
}

type Result struct {
	ID      int64   `json:"id"`
	Score   float64 `json:"score"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Host    string  `json:"host"`
	Lang    string  `json:"lang,omitempty"`
	Snippet string  `json:"snippet"`
}

func (s *Service) Search(ctx context.Context, req Request) (Response, error) {
	if s == nil || s.Backend == nil { return Response{}, errors.New("search service is not initialized") }
	norm, err := querynorm.Normalize(req.Query)
	if err != nil { return Response{}, err }
	limit := req.Limit
	if limit <= 0 { limit = 10 }
	if limit > 20 { limit = 20 }
	cacheKey := norm.Primary + "|" + strconv.Itoa(limit)
	if s.Cache != nil {
		if cached, ok := s.Cache.Get(cacheKey); ok { cached.Cached = true; return cached, nil }
	}

	queries := append([]string{norm.Primary}, norm.Variants...)
	var found backend.Result
	used := norm.Primary
	for _, q := range queries {
		res, err := s.Backend.Search(ctx, q, minInt(limit*3, 50))
		if err != nil { return Response{}, err }
		found = res
		used = q
		if len(res.Hits) > 0 { break }
	}

	reranked := rerank(found.Hits)
	resp := Response{Query: req.Query, Normalized: norm.Primary, UsedQuery: used, Total: found.Total, TookMS: found.TookMS}
	resp.Results = diversify(reranked, limit, 2)
	if s.Cache != nil { s.Cache.Put(cacheKey, resp, 30*time.Second) }
	return resp, nil
}

// rerank applies bounded quality/spam guardrails to the lexical score. The
// lexical score remains dominant: quality can add at most 20%, while severe
// spam can demote a result down to 20% of its lexical score.
func rerank(hits []backend.Hit) []backend.Hit {
	out := append([]backend.Hit(nil), hits...)
	for i := range out {
		factor := 1 + 0.002*out[i].QualityScore - 0.008*out[i].SpamScore
		if factor < 0.20 { factor = 0.20 }
		if factor > 1.20 { factor = 1.20 }
		out[i].Score *= factor
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func diversify(hits []backend.Hit, limit, perHost int) []Result {
	if limit <= 0 || perHost <= 0 { return nil }
	counts := make(map[string]int)
	out := make([]Result, 0, limit)
	for _, h := range hits {
		host := strings.ToLower(strings.TrimSpace(h.Host))
		if host != "" && counts[host] >= perHost { continue }
		if host != "" { counts[host]++ }
		out = append(out, Result{ID:h.ID, Score:h.Score, Title:h.Title, URL:h.URL, Host:h.Host, Lang:h.Lang, Snippet:h.Snippet})
		if len(out) >= limit { break }
	}
	return out
}

func minInt(a, b int) int { if a < b { return a }; return b }
