package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const GoldenSchemaVersion = 1

type GoldenSet struct {
	Version int           `json:"version"`
	Queries []GoldenQuery `json:"queries"`
}

type GoldenQuery struct {
	ID        string     `json:"id"`
	Query     string     `json:"query"`
	Tags      []string   `json:"tags,omitempty"`
	Judgments []Judgment `json:"judgments"`
}

type Judgment struct {
	URL   string `json:"url"`
	Grade int    `json:"grade"`
}

func LoadGolden(r io.Reader) (GoldenSet, error) {
	if r == nil {
		return GoldenSet{}, errors.New("golden reader is nil")
	}
	decoder := json.NewDecoder(io.LimitReader(r, 8<<20))
	decoder.DisallowUnknownFields()
	var set GoldenSet
	if err := decoder.Decode(&set); err != nil {
		return GoldenSet{}, fmt.Errorf("decode golden set: %w", err)
	}
	if err := set.Validate(); err != nil {
		return GoldenSet{}, err
	}
	return set, nil
}

func (s GoldenSet) Validate() error {
	if s.Version != GoldenSchemaVersion {
		return fmt.Errorf("unsupported golden schema version %d", s.Version)
	}
	if len(s.Queries) == 0 {
		return errors.New("golden set has no queries")
	}
	seenIDs := make(map[string]struct{}, len(s.Queries))
	for i, q := range s.Queries {
		q.ID = strings.TrimSpace(q.ID)
		q.Query = strings.TrimSpace(q.Query)
		if q.ID == "" || q.Query == "" {
			return fmt.Errorf("query %d requires id and query", i)
		}
		if _, ok := seenIDs[q.ID]; ok {
			return fmt.Errorf("duplicate golden query id %q", q.ID)
		}
		seenIDs[q.ID] = struct{}{}
		seenURLs := make(map[string]struct{}, len(q.Judgments))
		for j, judgment := range q.Judgments {
			if judgment.Grade < 0 || judgment.Grade > 3 {
				return fmt.Errorf("query %q judgment %d has invalid grade %d", q.ID, j, judgment.Grade)
			}
			normalized, err := NormalizeJudgmentURL(judgment.URL)
			if err != nil {
				return fmt.Errorf("query %q judgment %d: %w", q.ID, j, err)
			}
			if _, ok := seenURLs[normalized]; ok {
				return fmt.Errorf("query %q has duplicate judged URL %q", q.ID, normalized)
			}
			seenURLs[normalized] = struct{}{}
		}
	}
	return nil
}

func NormalizeJudgmentURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("judgment URL must be absolute http/https URL")
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}
