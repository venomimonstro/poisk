package extractor

import (
	"bytes"
	"errors"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var (
	ErrInputTooLarge = errors.New("html input exceeds hard limit")
	ErrTooManyNodes  = errors.New("html node count exceeds hard limit")
)

type Limits struct {
	MaxInputBytes          int
	MaxNodes               int
	MaxTextChars           int
	MaxStructuredItems     int
	MaxStructuredItemBytes int
}

func DefaultLimits() Limits {
	return Limits{
		MaxInputBytes:          8 << 20,
		MaxNodes:               250000,
		MaxTextChars:           2 << 20,
		MaxStructuredItems:     32,
		MaxStructuredItemBytes: 256 << 10,
	}
}

type RobotsDirectives struct {
	NoIndex  bool
	NoFollow bool
}

type Canonical struct {
	Raw      string
	Resolved string
	Valid    bool
	SameHost bool
}

type Document struct {
	Title                  string
	Description            string
	Lang                   string
	Canonical              Canonical
	Robots                  RobotsDirectives
	Text                    string
	StructuredData          []string
	StructuredDataTruncated bool
}

func Parse(input []byte, baseURL string, limits Limits) (Document, error) {
	limits = normalizeLimits(limits)
	if len(input) > limits.MaxInputBytes {
		return Document{}, ErrInputTooLarge
	}

	root, err := html.Parse(bytes.NewReader(input))
	if err != nil {
		return Document{}, err
	}

	var doc Document
	var textParts []string
	nodes := 0
	textChars := 0

	var walk func(*html.Node, bool) error
	walk = func(n *html.Node, hidden bool) error {
		nodes++
		if nodes > limits.MaxNodes {
			return ErrTooManyNodes
		}

		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			hidden = hidden || isHiddenElement(n, tag)

			switch tag {
			case "html":
				if doc.Lang == "" {
					doc.Lang = normalizeSpace(attr(n, "lang"))
				}
			case "title":
				if doc.Title == "" {
					doc.Title = normalizeSpace(descendantText(n, 4096))
				}
			case "meta":
				name := strings.ToLower(strings.TrimSpace(attr(n, "name")))
				content := normalizeSpace(attr(n, "content"))
				if name == "description" && doc.Description == "" {
					doc.Description = content
				}
				if name == "robots" || name == "poiskbot" {
					applyRobots(&doc.Robots, content)
				}
			case "link":
				if doc.Canonical.Raw == "" && hasRel(attr(n, "rel"), "canonical") {
					doc.Canonical = ResolveCanonical(attr(n, "href"), baseURL)
				}
			case "script":
				if strings.EqualFold(strings.TrimSpace(attr(n, "type")), "application/ld+json") {
					raw := strings.TrimSpace(descendantText(n, limits.MaxStructuredItemBytes+1))
					if raw != "" {
						if len(raw) > limits.MaxStructuredItemBytes || len(doc.StructuredData) >= limits.MaxStructuredItems {
							doc.StructuredDataTruncated = true
						} else {
							doc.StructuredData = append(doc.StructuredData, raw)
						}
					}
				}
			}
		}

		if n.Type == html.TextNode && !hidden && !isMetadataParent(n.Parent) {
			part := normalizeSpace(n.Data)
			if part != "" && textChars < limits.MaxTextChars {
				remaining := limits.MaxTextChars - textChars
				part = truncateRunes(part, remaining)
				if part != "" {
					textParts = append(textParts, part)
					textChars += len([]rune(part))
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c, hidden); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(root, false); err != nil {
		return Document{}, err
	}
	doc.Text = normalizeSpace(strings.Join(textParts, " "))
	return doc, nil
}

func ResolveCanonical(raw, baseURL string) Canonical {
	raw = strings.TrimSpace(raw)
	c := Canonical{Raw: raw}
	if raw == "" {
		return c
	}

	u, err := url.Parse(raw)
	if err != nil {
		return c
	}
	var base *url.URL
	if baseURL != "" {
		base, _ = url.Parse(baseURL)
		if base != nil {
			u = base.ResolveReference(u)
		}
	}
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return c
	}
	u.Fragment = ""
	c.Resolved = u.String()
	c.Valid = true
	if base != nil && base.Hostname() != "" {
		c.SameHost = strings.EqualFold(strings.TrimSuffix(base.Hostname(), "."), strings.TrimSuffix(u.Hostname(), "."))
	}
	return c
}

func normalizeLimits(l Limits) Limits {
	d := DefaultLimits()
	if l.MaxInputBytes <= 0 { l.MaxInputBytes = d.MaxInputBytes }
	if l.MaxNodes <= 0 { l.MaxNodes = d.MaxNodes }
	if l.MaxTextChars <= 0 { l.MaxTextChars = d.MaxTextChars }
	if l.MaxStructuredItems <= 0 { l.MaxStructuredItems = d.MaxStructuredItems }
	if l.MaxStructuredItemBytes <= 0 { l.MaxStructuredItemBytes = d.MaxStructuredItemBytes }
	return l
}

func isHiddenElement(n *html.Node, tag string) bool {
	switch tag {
	case "script", "style", "noscript", "template", "svg", "canvas", "nav", "footer", "form", "button", "select", "option":
		return true
	}
	if hasAttr(n, "hidden") || strings.EqualFold(strings.TrimSpace(attr(n, "aria-hidden")), "true") {
		return true
	}
	style := strings.ToLower(strings.ReplaceAll(attr(n, "style"), " ", ""))
	return strings.Contains(style, "display:none") || strings.Contains(style, "visibility:hidden")
}

func isMetadataParent(n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p.Type != html.ElementNode { continue }
		switch strings.ToLower(p.Data) {
		case "head", "script", "style", "noscript", "template", "svg", "canvas":
			return true
		}
	}
	return false
}

func applyRobots(dst *RobotsDirectives, content string) {
	for _, token := range strings.FieldsFunc(strings.ToLower(content), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		switch token {
		case "noindex":
			dst.NoIndex = true
		case "nofollow":
			dst.NoFollow = true
		case "none":
			dst.NoIndex, dst.NoFollow = true, true
		}
	}
}

func hasRel(value, wanted string) bool {
	for _, token := range strings.Fields(strings.ToLower(value)) {
		if token == wanted { return true }
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) { return a.Val }
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) { return true }
	}
	return false
}

func descendantText(n *html.Node, limit int) string {
	if n == nil || limit <= 0 { return "" }
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if b.Len() > limit { return }
		if cur.Type == html.TextNode { b.WriteString(cur.Data) }
		for c := cur.FirstChild; c != nil; c = c.NextSibling { walk(c) }
	}
	walk(n)
	return b.String()
}

func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func truncateRunes(s string, max int) string {
	if max <= 0 { return "" }
	r := []rune(s)
	if len(r) <= max { return s }
	return string(r[:max])
}
