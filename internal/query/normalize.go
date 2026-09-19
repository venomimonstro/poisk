package query

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrEmptyQuery    = errors.New("query is empty")
	ErrQueryTooLong  = errors.New("query exceeds hard limit")
	ErrInvalidQuery  = errors.New("query contains invalid utf-8")
)

const MaxQueryRunes = 256

type Normalized struct {
	Original string
	Primary  string
	Variants []string
}

func Normalize(raw string) (Normalized, error) {
	if !utf8.ValidString(raw) { return Normalized{}, ErrInvalidQuery }
	raw = strings.TrimSpace(raw)
	if raw == "" { return Normalized{}, ErrEmptyQuery }
	if utf8.RuneCountInString(raw) > MaxQueryRunes { return Normalized{}, ErrQueryTooLong }
	primary := normalizeText(raw)
	if primary == "" { return Normalized{}, ErrEmptyQuery }

	variants := make([]string, 0, 4)
	seen := map[string]struct{}{primary: {}}
	add := func(v string) {
		v = normalizeText(v)
		if v == "" { return }
		if _, ok := seen[v]; ok { return }
		seen[v] = struct{}{}
		variants = append(variants, v)
	}

	if looksWrongLayout(primary) {
		add(swapLayout(primary))
	}
	if containsCyrillic(primary) {
		add(transliterateRU(primary))
	}
	if containsLatin(primary) {
		add(transliterateLatinToRU(primary))
	}
	if len(variants) > 3 { variants = variants[:3] }
	return Normalized{Original: raw, Primary: primary, Variants: variants}, nil
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

func containsCyrillic(s string) bool {
	for _, r := range s { if unicode.In(r, unicode.Cyrillic) { return true } }
	return false
}

func containsLatin(s string) bool {
	for _, r := range s { if unicode.In(r, unicode.Latin) { return true } }
	return false
}
