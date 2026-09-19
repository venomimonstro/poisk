package answer

import (
	"context"
	"errors"
	"html"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	searchsvc "github.com/venomimonstro/poisk/internal/search"
)

const (
	DefaultMinConfidence = 0.58
	maxEvidenceRunes     = 420
	maxClaimRunes        = 320
	maxSources           = 4
)

type Searcher interface {
	Search(ctx context.Context, req searchsvc.Request) (searchsvc.Response, error)
}

type Service struct {
	Search        Searcher
	MinConfidence float64
}

type Request struct {
	Query string `json:"query"`
}

type Source struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Host  string `json:"host"`
}

type Claim struct {
	Text      string `json:"text"`
	SourceIDs []int  `json:"source_ids"`
}

type Response struct {
	Available      bool     `json:"available"`
	Answer         string   `json:"answer,omitempty"`
	Claims         []Claim  `json:"claims,omitempty"`
	Sources        []Source `json:"sources,omitempty"`
	Confidence     float64  `json:"confidence"`
	FallbackReason string   `json:"fallback_reason,omitempty"`
}

type evidence struct {
	result searchsvc.Result
	text   string
	claim  string
	score  float64
	rank   int
}

func (s *Service) Answer(ctx context.Context, req Request) (Response, error) {
	if s == nil || s.Search == nil { return Response{}, errors.New("answer service is not initialized") }
	searchResponse, err := s.Search.Search(ctx, searchsvc.Request{Query: req.Query, Limit: 10})
	if err != nil { return Response{}, err }
	return Build(req.Query, searchResponse.Results, s.threshold()), nil
}

func Build(query string, results []searchsvc.Result, minConfidence float64) Response {
	if minConfidence <= 0 || minConfidence > 1 { minConfidence = DefaultMinConfidence }
	queryTokens := tokens(query)
	if len(queryTokens) == 0 { return Response{FallbackReason: "invalid_query"} }

	candidates := make([]evidence, 0, len(results))
	seenHosts := make(map[string]struct{})
	for rank, result := range results {
		host := strings.ToLower(strings.TrimSpace(result.Host))
		if host == "" || result.URL == "" { continue }
		if _, exists := seenHosts[host]; exists { continue }
		text := cleanEvidence(result.Snippet)
		if utf8.RuneCountInString(text) < 24 { continue }
		claim := bestSentence(text, queryTokens)
		if utf8.RuneCountInString(claim) < 20 { continue }
		coverage := tokenCoverage(queryTokens, tokens(claim))
		rankScore := 1.0 / (1.0 + float64(rank)*0.18)
		score := clamp01(0.72*coverage + 0.28*rankScore)
		if score < 0.30 { continue }
		seenHosts[host] = struct{}{}
		candidates = append(candidates, evidence{result: result, text: text, claim: claim, score: score, rank: rank})
	}

	if len(candidates) < 2 { return Response{FallbackReason: "insufficient_independent_sources"} }
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score { return candidates[i].rank < candidates[j].rank }
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) > maxSources { candidates = candidates[:maxSources] }

	avg := 0.0
	for _, item := range candidates { avg += item.score }
	avg /= float64(len(candidates))
	sourceFactor := clamp01(float64(len(candidates)-1) / 2.0)
	confidence := clamp01(0.65*avg + 0.20*candidates[0].score + 0.15*sourceFactor)
	if confidence < minConfidence {
		return Response{Confidence: confidence, FallbackReason: "low_confidence"}
	}

	response := Response{Available: true, Confidence: confidence}
	response.Sources = make([]Source, 0, len(candidates))
	response.Claims = make([]Claim, 0, len(candidates))
	for i, item := range candidates {
		id := i + 1
		response.Sources = append(response.Sources, Source{ID: id, Title: item.result.Title, URL: item.result.URL, Host: item.result.Host})
		response.Claims = append(response.Claims, Claim{Text: item.claim, SourceIDs: []int{id}})
	}
	// Answer is purely extractive. No factual connective text is synthesized.
	response.Answer = response.Claims[0].Text
	return response
}

func (s *Service) threshold() float64 {
	if s.MinConfidence <= 0 || s.MinConfidence > 1 { return DefaultMinConfidence }
	return s.MinConfidence
}

func cleanEvidence(raw string) string {
	raw = strings.ReplaceAll(raw, "[[", "")
	raw = strings.ReplaceAll(raw, "]]", "")
	raw = html.UnescapeString(raw)
	var b strings.Builder
	insideTag := false
	for _, r := range raw {
		switch r {
		case '<': insideTag = true
		case '>': insideTag = false
		default:
			if !insideTag { b.WriteRune(r) }
		}
	}
	return truncateRunes(strings.Join(strings.Fields(b.String()), " "), maxEvidenceRunes)
}

func bestSentence(text string, queryTokens map[string]struct{}) string {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '\n' || r == '\r' })
	best := ""
	bestScore := -1.0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if utf8.RuneCountInString(part) < 20 { continue }
		score := tokenCoverage(queryTokens, tokens(part))
		if score > bestScore {
			best, bestScore = part, score
		}
	}
	if best == "" { best = text }
	return truncateRunes(best, maxClaimRunes)
}

func tokens(s string) map[string]struct{} {
	parts := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := make(map[string]struct{}, len(parts))
	for _, token := range parts {
		if utf8.RuneCountInString(token) < 2 { continue }
		out[token] = struct{}{}
	}
	return out
}

func tokenCoverage(query, text map[string]struct{}) float64 {
	if len(query) == 0 { return 0 }
	matched := 0
	for token := range query { if _, ok := text[token]; ok { matched++ } }
	return float64(matched) / float64(len(query))
}

func truncateRunes(s string, max int) string {
	if max <= 0 { return "" }
	runes := []rune(s)
	if len(runes) <= max { return s }
	return string(runes[:max])
}

func clamp01(v float64) float64 {
	if v < 0 { return 0 }
	if v > 1 { return 1 }
	return v
}
