package sitemap

import (
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrTooLarge = errors.New("sitemap exceeds decompressed size limit")
	ErrTooMany  = errors.New("sitemap exceeds item limit")
	ErrTooDeep  = errors.New("sitemap recursion depth exceeds limit")
	ErrUnknown  = errors.New("unknown sitemap document")
)

type Limits struct {
	MaxCompressedBytes   int64
	MaxDecompressedBytes int64
	MaxURLs              int
	MaxSitemaps          int
	MaxDepth             int
}

func DefaultLimits() Limits {
	return Limits{
		MaxCompressedBytes:   8 << 20,
		MaxDecompressedBytes: 32 << 20,
		MaxURLs:              50_000,
		MaxSitemaps:          1_000,
		MaxDepth:             3,
	}
}

type Kind string

const (
	KindURLSet       Kind = "urlset"
	KindSitemapIndex Kind = "sitemapindex"
)

type Result struct {
	Kind     Kind
	URLs     []string
	Sitemaps []string
}

type countingReader struct {
	r     io.Reader
	n     int64
	limit int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.n >= c.limit {
		return 0, ErrTooLarge
	}
	remaining := c.limit - c.n
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.limit {
		return n, ErrTooLarge
	}
	return n, err
}

func Parse(r io.Reader, compressed bool, depth int, limits Limits) (Result, error) {
	if limits.MaxCompressedBytes <= 0 || limits.MaxDecompressedBytes <= 0 || limits.MaxURLs <= 0 || limits.MaxSitemaps <= 0 || limits.MaxDepth < 0 {
		return Result{}, errors.New("invalid sitemap limits")
	}
	if depth > limits.MaxDepth {
		return Result{}, ErrTooDeep
	}

	compressedReader := io.LimitReader(r, limits.MaxCompressedBytes+1)
	var decoded io.Reader = compressedReader
	if compressed {
		gz, err := gzip.NewReader(compressedReader)
		if err != nil {
			return Result{}, fmt.Errorf("open gzip sitemap: %w", err)
		}
		defer gz.Close()
		decoded = gz
	}
	cr := &countingReader{r: decoded, limit: limits.MaxDecompressedBytes}
	dec := xml.NewDecoder(cr)
	dec.Strict = true

	var result Result
	var rootSeen bool
	var inURL, inSitemap, inLoc bool
	var text strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, ErrTooLarge) {
				return Result{}, ErrTooLarge
			}
			return Result{}, fmt.Errorf("decode sitemap XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if !rootSeen {
				rootSeen = true
				switch name {
				case "urlset":
					result.Kind = KindURLSet
				case "sitemapindex":
					result.Kind = KindSitemapIndex
				default:
					return Result{}, ErrUnknown
				}
			}
			switch name {
			case "url":
				inURL = true
			case "sitemap":
				inSitemap = true
			case "loc":
				if inURL || inSitemap {
					inLoc = true
					text.Reset()
				}
			}
		case xml.CharData:
			if inLoc {
				text.Write([]byte(t))
			}
		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			if name == "loc" && inLoc {
				loc := strings.TrimSpace(text.String())
				if loc != "" {
					if inURL {
						if result.Kind != KindURLSet {
							return Result{}, ErrUnknown
						}
						if len(result.URLs) >= limits.MaxURLs {
							return Result{}, ErrTooMany
						}
						result.URLs = append(result.URLs, loc)
					} else if inSitemap {
						if result.Kind != KindSitemapIndex {
							return Result{}, ErrUnknown
						}
						if len(result.Sitemaps) >= limits.MaxSitemaps {
							return Result{}, ErrTooMany
						}
						result.Sitemaps = append(result.Sitemaps, loc)
					}
				}
				inLoc = false
				text.Reset()
			}
			if name == "url" {
				inURL = false
			}
			if name == "sitemap" {
				inSitemap = false
			}
		}
	}

	if !rootSeen || result.Kind == "" {
		return Result{}, ErrUnknown
	}
	return result, nil
}
