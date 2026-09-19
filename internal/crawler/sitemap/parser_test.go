package sitemap

import (
	"bytes"
	"compress/gzip"
	"errors"
	"strings"
	"testing"
)

func TestParseURLSet(t *testing.T) {
	xml := `<?xml version="1.0"?><urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url></urlset>`
	got, err := Parse(strings.NewReader(xml), false, 0, DefaultLimits())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Kind != KindURLSet || len(got.URLs) != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestParseSitemapIndex(t *testing.T) {
	xml := `<sitemapindex><sitemap><loc>https://example.com/s1.xml</loc></sitemap><sitemap><loc>https://example.com/s2.xml.gz</loc></sitemap></sitemapindex>`
	got, err := Parse(strings.NewReader(xml), false, 1, DefaultLimits())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Kind != KindSitemapIndex || len(got.Sitemaps) != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestParseGzip(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(`<urlset><url><loc>https://example.com/a</loc></url></urlset>`))
	_ = gz.Close()

	got, err := Parse(bytes.NewReader(buf.Bytes()), true, 0, DefaultLimits())
	if err != nil || len(got.URLs) != 1 {
		t.Fatalf("gzip parse result=%+v err=%v", got, err)
	}
}

func TestSitemapLimits(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxURLs = 1
	xml := `<urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url></urlset>`
	if _, err := Parse(strings.NewReader(xml), false, 0, limits); !errors.Is(err, ErrTooMany) {
		t.Fatalf("expected ErrTooMany, got %v", err)
	}

	limits = DefaultLimits()
	limits.MaxDecompressedBytes = 20
	if _, err := Parse(strings.NewReader(`<urlset><url><loc>https://example.com/a</loc></url></urlset>`), false, 0, limits); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}

	limits = DefaultLimits()
	limits.MaxDepth = 2
	if _, err := Parse(strings.NewReader(`<urlset></urlset>`), false, 3, limits); !errors.Is(err, ErrTooDeep) {
		t.Fatalf("expected ErrTooDeep, got %v", err)
	}
}

func TestMalformedAndUnknownXML(t *testing.T) {
	for _, raw := range []string{`<root></root>`, `<urlset><url>`} {
		if _, err := Parse(strings.NewReader(raw), false, 0, DefaultLimits()); err == nil {
			t.Fatalf("expected parse failure for %q", raw)
		}
	}
}
