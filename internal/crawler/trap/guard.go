package trap

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrURLTooLong       = errors.New("URL exceeds hard length limit")
	ErrPathTooDeep      = errors.New("URL path exceeds depth limit")
	ErrTooManyParams    = errors.New("URL contains too many query parameters")
	ErrSessionParameter = errors.New("URL contains session parameter")
	ErrSearchPage       = errors.New("internal search page is not crawlable")
	ErrRepeatedSegments = errors.New("URL contains repeated path segments")
)

type Limits struct {
	MaxURLLength   int
	MaxPathDepth   int
	MaxQueryParams int
}

func DefaultLimits() Limits {
	return Limits{MaxURLLength: 4096, MaxPathDepth: 16, MaxQueryParams: 20}
}

var sessionKeys = map[string]struct{}{
	"sid": {}, "sessionid": {}, "phpsessid": {}, "jsessionid": {},
}

var searchKeys = map[string]struct{}{
	"q": {}, "query": {}, "search": {}, "keyword": {}, "keywords": {},
}

func Check(raw string, limits Limits) error {
	if len(raw) > limits.MaxURLLength {
		return ErrURLTooLong
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	segments := pathSegments(u.Path)
	if len(segments) > limits.MaxPathDepth {
		return ErrPathTooDeep
	}
	if hasRepeatedSegmentRun(segments, 3) {
		return ErrRepeatedSegments
	}

	q := u.Query()
	if len(q) > limits.MaxQueryParams {
		return ErrTooManyParams
	}
	for key := range q {
		lower := strings.ToLower(key)
		if _, ok := sessionKeys[lower]; ok {
			return ErrSessionParameter
		}
	}

	path := strings.ToLower(strings.Trim(u.Path, "/"))
	looksLikeSearch := path == "search" || strings.HasPrefix(path, "search/") || strings.Contains(path, "/search/")
	if looksLikeSearch {
		for key := range q {
			if _, ok := searchKeys[strings.ToLower(key)]; ok {
				return ErrSearchPage
			}
		}
	}
	return nil
}

func pathSegments(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasRepeatedSegmentRun(parts []string, threshold int) bool {
	if threshold <= 1 {
		return len(parts) > 0
	}
	counts := make(map[string]int, len(parts))
	for _, part := range parts {
		key := strings.ToLower(part)
		counts[key]++
		if counts[key] >= threshold {
			return true
		}
	}
	return false
}
