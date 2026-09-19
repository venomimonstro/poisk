package extractor

import (
	"errors"
	"strings"
	"testing"
)

func TestParseExtractsMetadataAndVisibleText(t *testing.T) {
	html := `<!doctype html><html lang="ru"><head><title>  Тестовая страница </title><meta name="description" content=" Короткое описание "><meta name="robots" content="noindex, nofollow"><link rel="canonical alternate" href="/canonical"><script>secret script text</script><style>.x{display:none}</style><script type="application/ld+json">{"@type":"Article","name":"Demo"}</script></head><body><nav>Навигация</nav><h1>Главный заголовок</h1><p>Полезный текст страницы.</p><div hidden>Скрытый текст</div><footer>Подвал</footer></body></html>`
	doc, err := Parse([]byte(html), "https://example.com/page", DefaultLimits())
	if err != nil { t.Fatal(err) }
	if doc.Title != "Тестовая страница" { t.Fatalf("title=%q", doc.Title) }
	if doc.Description != "Короткое описание" { t.Fatalf("description=%q", doc.Description) }
	if doc.Lang != "ru" { t.Fatalf("lang=%q", doc.Lang) }
	if !doc.Robots.NoIndex || !doc.Robots.NoFollow { t.Fatalf("robots=%+v", doc.Robots) }
	if !doc.Canonical.Valid || !doc.Canonical.SameHost || doc.Canonical.Resolved != "https://example.com/canonical" { t.Fatalf("canonical=%+v", doc.Canonical) }
	if !strings.Contains(doc.Text, "Главный заголовок") || !strings.Contains(doc.Text, "Полезный текст страницы.") { t.Fatalf("text=%q", doc.Text) }
	for _, unwanted := range []string{"secret script text", "Навигация", "Скрытый текст", "Подвал"} {
		if strings.Contains(doc.Text, unwanted) { t.Fatalf("text contains hidden/boilerplate %q: %q", unwanted, doc.Text) }
	}
	if len(doc.StructuredData) != 1 || !strings.Contains(doc.StructuredData[0], `"@type":"Article"`) { t.Fatalf("structured=%v", doc.StructuredData) }
}

func TestParseHardInputLimit(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxInputBytes = 16
	_, err := Parse([]byte(strings.Repeat("x", 17)), "https://example.com/", limits)
	if !errors.Is(err, ErrInputTooLarge) { t.Fatalf("error=%v", err) }
}

func TestParseNodeLimit(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxNodes = 5
	_, err := Parse([]byte(`<html><body><div><span>one</span><span>two</span></div></body></html>`), "https://example.com/", limits)
	if !errors.Is(err, ErrTooManyNodes) { t.Fatalf("error=%v", err) }
}

func TestStructuredDataLimits(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxStructuredItems = 1
	limits.MaxStructuredItemBytes = 16
	html := `<html><head><script type="application/ld+json">{"a":1}</script><script type="application/ld+json">{"b":2}</script></head><body>ok</body></html>`
	doc, err := Parse([]byte(html), "https://example.com/", limits)
	if err != nil { t.Fatal(err) }
	if len(doc.StructuredData) != 1 || !doc.StructuredDataTruncated { t.Fatalf("structured=%v truncated=%v", doc.StructuredData, doc.StructuredDataTruncated) }
}

func TestResolveCanonicalDoesNotBlindlyTrustTarget(t *testing.T) {
	cross := ResolveCanonical("https://other.example/a#frag", "https://example.com/page")
	if !cross.Valid || cross.SameHost || cross.Resolved != "https://other.example/a" { t.Fatalf("cross=%+v", cross) }
	bad := ResolveCanonical("file:///etc/passwd", "https://example.com/page")
	if bad.Valid { t.Fatalf("bad canonical marked valid: %+v", bad) }
	withUser := ResolveCanonical("https://user:pass@example.com/a", "https://example.com/page")
	if withUser.Valid { t.Fatalf("userinfo canonical marked valid: %+v", withUser) }
}

func TestEmptyAndThinHTML(t *testing.T) {
	for _, input := range []string{"", "<html></html>", "<p>ok</p>"} {
		doc, err := Parse([]byte(input), "https://example.com/", DefaultLimits())
		if err != nil { t.Fatalf("input=%q err=%v", input, err) }
		if input == "<p>ok</p>" && doc.Text != "ok" { t.Fatalf("text=%q", doc.Text) }
	}
}
