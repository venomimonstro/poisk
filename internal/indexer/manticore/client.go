package manticore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrResponseTooLarge = errors.New("manticore response exceeds hard limit")
	ErrRemote           = errors.New("manticore returned an error")
)

type Config struct {
	BaseURL          string
	RequestTimeout   time.Duration
	MaxResponseBytes int64
}

func DefaultConfig() Config {
	return Config{
		BaseURL:          "http://manticore:9308",
		RequestTimeout:   5 * time.Second,
		MaxResponseBytes: 1 << 20,
	}
}

type Client struct {
	baseURL string
	maxBody int64
	http    *http.Client
}

func New(cfg Config) (*Client, error) {
	d := DefaultConfig()
	if cfg.BaseURL == "" { cfg.BaseURL = d.BaseURL }
	if cfg.RequestTimeout <= 0 { cfg.RequestTimeout = d.RequestTimeout }
	if cfg.MaxResponseBytes <= 0 { cfg.MaxResponseBytes = d.MaxResponseBytes }
	u, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("invalid manticore base URL")
	}
	return &Client{
		baseURL: u.String(),
		maxBody: cfg.MaxResponseBytes,
		http: &http.Client{Timeout: cfg.RequestTimeout},
	}, nil
}

func (c *Client) EnsureSchema(ctx context.Context) error {
	_, err := c.execSQL(ctx, WebSchemaSQL)
	return err
}

func (c *Client) Apply(ctx context.Context, doc Document) (bool, error) {
	if doc.ID <= 0 || doc.EntityVersion <= 0 {
		return false, errors.New("document id and entity version must be positive")
	}
	current, exists, err := c.CurrentVersion(ctx, doc.ID)
	if err != nil { return false, err }
	if exists && current > doc.EntityVersion {
		return false, nil
	}
	q := "REPLACE INTO " + WebIndex + " (id,title,description,body,url,host,lang,content_hash,entity_version,quality_score,spam_score,fetched_at) VALUES (" +
		strconv.FormatInt(doc.ID, 10) + "," +
		quote(doc.Title) + "," + quote(doc.Description) + "," + quote(doc.Body) + "," +
		quote(doc.URL) + "," + quote(doc.Host) + "," + quote(doc.Lang) + "," + quote(doc.ContentHash) + "," +
		strconv.FormatInt(doc.EntityVersion, 10) + "," +
		strconv.FormatFloat(doc.QualityScore, 'f', -1, 64) + "," +
		strconv.FormatFloat(doc.SpamScore, 'f', -1, 64) + "," +
		strconv.FormatInt(doc.FetchedAtUnix, 10) + ")"
	_, err = c.execSQL(ctx, q)
	return err == nil, err
}

func (c *Client) Delete(ctx context.Context, id int64) error {
	if id <= 0 { return errors.New("document id must be positive") }
	_, err := c.execSQL(ctx, "DELETE FROM "+WebIndex+" WHERE id="+strconv.FormatInt(id, 10))
	return err
}

type rawResultSet struct {
	Data  []map[string]json.RawMessage `json:"data"`
	Error string                       `json:"error"`
}

func (c *Client) CurrentVersion(ctx context.Context, id int64) (int64, bool, error) {
	if id <= 0 { return 0, false, errors.New("document id must be positive") }
	body, err := c.execSQL(ctx, "SELECT entity_version FROM "+WebIndex+" WHERE id="+strconv.FormatInt(id, 10)+" LIMIT 1")
	if err != nil { return 0, false, err }
	var sets []rawResultSet
	if err := json.Unmarshal(body, &sets); err != nil {
		return 0, false, fmt.Errorf("decode current version: %w", err)
	}
	if len(sets) == 0 { return 0, false, nil }
	if sets[0].Error != "" { return 0, false, fmt.Errorf("%w: %s", ErrRemote, sets[0].Error) }
	if len(sets[0].Data) == 0 { return 0, false, nil }
	raw, ok := sets[0].Data[0]["entity_version"]
	if !ok { return 0, false, errors.New("manticore version result missing entity_version") }
	var version int64
	if err := json.Unmarshal(raw, &version); err == nil { return version, true, nil }
	var text string
	if err := json.Unmarshal(raw, &text); err != nil { return 0, false, fmt.Errorf("decode entity_version: %w", err) }
	version, err = strconv.ParseInt(text, 10, 64)
	if err != nil { return 0, false, fmt.Errorf("parse entity_version: %w", err) }
	return version, true, nil
}

func (c *Client) execSQL(ctx context.Context, query string) ([]byte, error) {
	if c == nil || c.http == nil { return nil, errors.New("manticore client is not initialized") }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sql?mode=raw", strings.NewReader(query))
	if err != nil { return nil, err }
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.ContentLength > c.maxBody && resp.ContentLength >= 0 { return nil, ErrResponseTooLarge }
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil { return nil, err }
	if int64(len(body)) > c.maxBody { return nil, ErrResponseTooLarge }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrRemote, resp.StatusCode, truncate(string(body), 512))
	}
	return body, nil
}

func quote(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		"'", "\\'",
		"\x00", "",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return "'" + r.Replace(s) + "'"
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n]
}
