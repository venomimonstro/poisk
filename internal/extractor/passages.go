package extractor

import "strings"

type PassageConfig struct {
	MaxChars    int
	MaxPassages int
}

func DefaultPassageConfig() PassageConfig {
	return PassageConfig{MaxChars: 800, MaxPassages: 128}
}

type Passage struct {
	Ordinal int
	Text    string
}

func SplitPassages(text string, cfg PassageConfig) []Passage {
	if cfg.MaxChars <= 0 { cfg.MaxChars = DefaultPassageConfig().MaxChars }
	if cfg.MaxPassages <= 0 { cfg.MaxPassages = DefaultPassageConfig().MaxPassages }
	words := strings.Fields(text)
	if len(words) == 0 { return nil }

	out := make([]Passage, 0, minInt(cfg.MaxPassages, 16))
	var current strings.Builder
	flush := func() bool {
		value := strings.TrimSpace(current.String())
		current.Reset()
		if value == "" { return true }
		if len(out) >= cfg.MaxPassages { return false }
		out = append(out, Passage{Ordinal: len(out), Text: value})
		return len(out) < cfg.MaxPassages
	}

	for _, word := range words {
		for len([]rune(word)) > cfg.MaxChars {
			if current.Len() > 0 && !flush() { return out }
			r := []rune(word)
			out = append(out, Passage{Ordinal: len(out), Text: string(r[:cfg.MaxChars])})
			if len(out) >= cfg.MaxPassages { return out }
			word = string(r[cfg.MaxChars:])
		}

		candidateLen := len([]rune(word))
		if current.Len() > 0 { candidateLen += len([]rune(current.String())) + 1 }
		if candidateLen > cfg.MaxChars {
			if !flush() { return out }
		}
		if current.Len() > 0 { current.WriteByte(' ') }
		current.WriteString(word)
	}
	flush()
	return out
}

func minInt(a, b int) int { if a < b { return a }; return b }
