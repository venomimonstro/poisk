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
	ErrResponseTooLarge = errors.New("search response exceeds hard limit")
	ErrSearchBackend    = errors.New("search backend error")
)

type Config struct {
	BaseURL          string
	Timeout          time.Duration
	MaxResponseBytes int64
	MaxResults       int
}

func DefaultConfig() Config {
	return Config{BaseURL: "http://manticore:9308", Timeout: 1200 * time.Millisecond, MaxResponseBytes: 2 << 20, MaxResults: 50}
}

type Client struct {
	baseURL string
	timeout time.Duration
	maxBody int64
	maxResults int
	http *http.Client
}

type Hit struct {
	ID          int64
	Score       float64
	Title       string
	Description string
	URL         string
	Host        string
	Lang        string
	Snippet     string
}

type Result struct {
	Total int64
	TookMS int64
	Hits []Hit
}

func New(cfg Config) (*Client, error) {
	d := DefaultConfig()
	if cfg.BaseURL == "" { cfg.BaseURL = d.BaseURL }
	if cfg.Timeout <= 0 { cfg.Timeout = d.Timeout }
	if cfg.MaxResponseBytes <= 0 { cfg.MaxResponseBytes = d.MaxResponseBytes }
	if cfg.MaxResults <= 0 { cfg.MaxResults = d.MaxResults }
	u, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" { return nil, errors.New("invalid manticore base URL") }
	h := &http.Client{Timeout: cfg.Timeout}
	h.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{baseURL: u.String(), timeout: cfg.Timeout, maxBody: cfg.MaxResponseBytes, maxResults: cfg.MaxResults, http: h}, nil
}

func (c *Client) Search(ctx context.Context, q string, limit int) (Result, error) {
	if c == nil || c.http == nil { return Result{}, errors.New("search client is not initialized") }
	if limit <= 0 { limit = 10 }
	if limit > c.maxResults { limit = c.maxResults }
	payload := map[string]any{
		"table": "web_documents",
		"query": map[string]any{"match": map[string]any{"title,description,body": q}},
		"limit": limit,
		"_source": []string{"title","description","url","host","lang"},
		"highlight": map[string]any{
			"fields": []string{"title","description","body"},
			"before_match": "[[",
			"after_match": "]]",
			"limit": 280,
		},
		"options": map[string]any{
			"ranker": "bm25",
			"field_weights": map[string]int{"title": 12, "description": 4, "body": 1},
			"max_matches": 1000,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil { return Result{}, err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(body))
	if err != nil { return Result{}, err }
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil { return Result{}, err }
	defer resp.Body.Close()
	if resp.ContentLength > c.maxBody && resp.ContentLength >= 0 { return Result{}, ErrResponseTooLarge }
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil { return Result{}, err }
	if int64(len(raw)) > c.maxBody { return Result{}, ErrResponseTooLarge }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return Result{}, fmt.Errorf("%w: status=%d", ErrSearchBackend, resp.StatusCode) }

	var decoded struct {
		Took int64 `json:"took"`
		TimedOut bool `json:"timed_out"`
		Hits struct {
			Total int64 `json:"total"`
			Hits []struct {
				ID int64 `json:"_id"`
				Score float64 `json:"_score"`
				Source struct {
					Title string `json:"title"`
					Description string `json:"description"`
					URL string `json:"url"`
					Host string `json:"host"`
					Lang string `json:"lang"`
				} `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil { return Result{}, fmt.Errorf("decode search response: %w", err) }
	if decoded.TimedOut { return Result{}, fmt.Errorf("%w: backend timeout", ErrSearchBackend) }
	out := Result{Total: decoded.Hits.Total, TookMS: decoded.Took, Hits: make([]Hit, 0, len(decoded.Hits.Hits))}
	for _, h := range decoded.Hits.Hits {
		snippet := firstSnippet(h.Highlight, h.Source.Description)
		out.Hits = append(out.Hits, Hit{ID:h.ID, Score:h.Score, Title:h.Source.Title, Description:h.Source.Description, URL:h.Source.URL, Host:h.Source.Host, Lang:h.Source.Lang, Snippet:snippet})
	}
	return out, nil
}

func firstSnippet(highlight map[string][]string, fallback string) string {
	for _, field := range []string{"description","body","title"} {
		if values := highlight[field]; len(values) > 0 && strings.TrimSpace(values[0]) != "" { return values[0] }
	}
	return fallback
}
