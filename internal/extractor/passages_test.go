package extractor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitPassagesRespectsRuneLimitAndOrdering(t *testing.T) {
	cfg := PassageConfig{MaxChars: 12, MaxPassages: 10}
	got := SplitPassages("раз два три четыре пять шесть", cfg)
	if len(got) < 2 { t.Fatalf("passages=%v", got) }
	for i, p := range got {
		if p.Ordinal != i { t.Fatalf("ordinal=%d want=%d", p.Ordinal, i) }
		if len([]rune(p.Text)) > cfg.MaxChars { t.Fatalf("passage too long: %q", p.Text) }
		if !utf8.ValidString(p.Text) { t.Fatalf("invalid utf8: %q", p.Text) }
	}
	joined := strings.Join(func() []string { out := make([]string, 0, len(got)); for _, p := range got { out = append(out, p.Text) }; return out }(), " ")
	if strings.Join(strings.Fields(joined), " ") != strings.Join(strings.Fields("раз два три четыре пять шесть"), " ") { t.Fatalf("joined=%q", joined) }
}

func TestSplitPassagesSplitsOversizedWordWithoutBreakingUTF8(t *testing.T) {
	got := SplitPassages("абвгдежзий", PassageConfig{MaxChars: 3, MaxPassages: 10})
	if len(got) != 4 { t.Fatalf("passages=%v", got) }
	if got[0].Text != "абв" || got[3].Text != "й" { t.Fatalf("passages=%v", got) }
	for _, p := range got {
		if !utf8.ValidString(p.Text) { t.Fatalf("invalid utf8: %q", p.Text) }
	}
}

func TestSplitPassagesHonorsCountLimit(t *testing.T) {
	got := SplitPassages("one two three four five six seven eight", PassageConfig{MaxChars: 4, MaxPassages: 2})
	if len(got) != 2 { t.Fatalf("passages=%v", got) }
}

func TestSplitPassagesEmpty(t *testing.T) {
	if got := SplitPassages("   \n\t ", DefaultPassageConfig()); got != nil { t.Fatalf("passages=%v", got) }
}
