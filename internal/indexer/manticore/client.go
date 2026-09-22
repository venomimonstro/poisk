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
	client := &http.Client{Timeout: cfg.RequestTimeout}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{baseURL: u.String(), maxBody: cfg.MaxResponseBytes, http: client}, nil
}

func (c *Client) EnsureSchema(ctx context.Context) error {
	if _, err := c.execSQL(ctx, WebSchemaSQL); err != nil { return err }
	body, err := c.execSQL(ctx, "DESC "+WebIndex)
	if err != nil { return fmt.Errorf("describe web index: %w", err) }
	hasAuthority, err := rawHasColumn(body, "authority_score")
	if err != nil { return err }
	hasFetchedAt, err := rawHasColumn(body, "fetched_at")
	if err != nil { return err }
	if !hasAuthority {
		if _, err := c.execSQL(ctx, "ALTER TABLE "+WebIndex+" ADD COLUMN authority_score FLOAT"); err != nil {
			return fmt.Errorf("add authority_score attribute: %w", err)
		}
	}
	if !hasFetchedAt {
		if _, err := c.execSQL(ctx, "ALTER TABLE "+WebIndex+" ADD COLUMN fetched_at TIMESTAMP"); err != nil {
			return fmt.Errorf("add fetched_at attribute: %w", err)
		}
	}
	return nil
}

func (c *Client) Apply(ctx context.Context, doc Document) (bool, error) {
	if doc.ID <= 0 || doc.EntityVersion <= 0 {
		return false, errors.New("document id and entity version must be positive")
	}
	current, exists, err := c.CurrentVersion(ctx, doc.ID)
	if err != nil { return false, err }
	if exists && current > doc.EntityVersion { return false, nil }

	q := "REPLACE INTO " + WebIndex + " (id,title,description,body,url,host,lang,content_hash,entity_version,quality_score,spam_score,authority_score,fetched_at) VALUES (" +
		strconv.FormatInt(doc.ID, 10) + "," + quote(doc.Title) + "," + quote(doc.Description) + "," + quote(doc.Body) + "," +
		quote(doc.URL) + "," + quote(doc.Host) + "," + quote(doc.Lang) + "," + quote(doc.ContentHash) + "," +
		strconv.FormatInt(doc.EntityVersion, 10) + "," + strconv.FormatFloat(clamp100(doc.QualityScore), 'f', -1, 64) + "," +
		strconv.FormatFloat(clamp100(doc.SpamScore), 'f', -1, 64) + "," + strconv.FormatFloat(clamp100(doc.AuthorityScore), 'f', -1, 64) + "," +
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

func (c *Client) CountDocuments(ctx context.Context) (int64, error) {
	body, err := c.execSQL(ctx, "SELECT COUNT(*) AS count FROM "+WebIndex)
	if err != nil { return 0, err }
	var sets []rawResultSet
	if err := json.Unmarshal(body, &sets); err != nil { return 0, fmt.Errorf("decode document count: %w", err) }
	if len(sets) == 0 || len(sets[0].Data) == 0 { return 0, errors.New("manticore document count result is empty") }
	raw, ok := sets[0].Data[0]["count"]
	if !ok { return 0, errors.New("manticore document count result missing count") }
	var count int64
	if err := json.Unmarshal(raw, &count); err == nil {
		if count < 0 { return 0, errors.New("manticore document count is negative") }
		return count, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil { return 0, fmt.Errorf("decode document count value: %w", err) }
	count, err = strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || count < 0 { return 0, errors.New("invalid manticore document count") }
	return count, nil
}

func (c *Client) CurrentVersion(ctx context.Context, id int64) (int64, bool, error) {
	if id <= 0 { return 0, false, errors.New("document id must be positive") }
	body, err := c.execSQL(ctx, "SELECT entity_version FROM "+WebIndex+" WHERE id="+strconv.FormatInt(id, 10)+" LIMIT 1")
	if err != nil { return 0, false, err }
	var sets []rawResultSet
	if err := json.Unmarshal(body, &sets); err != nil { return 0, false, fmt.Errorf("decode current version: %w", err) }
	if len(sets) == 0 || len(sets[0].Data) == 0 { return 0, false, nil }
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

func rawHasColumn(body []byte, wanted string) (bool, error) {
	var sets []rawResultSet
	if err := json.Unmarshal(body, &sets); err != nil { return false, fmt.Errorf("decode manticore schema: %w", err) }
	if len(sets) == 0 { return false, errors.New("manticore schema response is empty") }
	for _, row := range sets[0].Data {
		for key, raw := range row {
			if !strings.EqualFold(key, "field") { continue }
			var field string
			if err := json.Unmarshal(raw, &field); err != nil { return false, fmt.Errorf("decode schema field: %w", err) }
			if strings.EqualFold(field, wanted) { return true, nil }
		}
	}
	return false, nil
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
	if err := validateRawResponse(body); err != nil { return nil, err }
	return body, nil
}

func validateRawResponse(body []byte) error {
	var sets []rawResultSet
	if err := json.Unmarshal(body, &sets); err != nil {
		return fmt.Errorf("decode manticore raw response: %w", err)
	}
	for _, set := range sets {
		if strings.TrimSpace(set.Error) != "" {
			return fmt.Errorf("%w: %s", ErrRemote, truncate(set.Error, 512))
		}
	}
	return nil
}

func quote(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "'", "\\'", "\x00", "", "\n", "\\n", "\r", "\\r", "\t", "\\t")
	return "'" + r.Replace(s) + "'"
}

func clamp100(v float64) float64 {
	if v < 0 { return 0 }
	if v > 100 { return 100 }
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n]
}
