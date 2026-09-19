package quality

import "math"

type DocumentSignals struct {
	TextRunes          int
	TitleRunes         int
	DescriptionRunes   int
	UniqueTokenRatio   float64
	ExternalLinkRatio  float64
	DuplicateRatio     float64
	BoilerplateRatio   float64
	HTTPS              bool
	HasStructuredData  bool
}

type DocumentScore struct {
	Quality float64 `json:"quality"`
	Spam    float64 `json:"spam"`
}

func ScoreDocument(s DocumentSignals) DocumentScore {
	unique := clamp01(s.UniqueTokenRatio)
	external := clamp01(s.ExternalLinkRatio)
	duplicate := clamp01(s.DuplicateRatio)
	boilerplate := clamp01(s.BoilerplateRatio)

	quality := 0.0
	quality += contentDepthScore(s.TextRunes) * 35
	quality += metadataScore(s.TitleRunes, s.DescriptionRunes) * 20
	quality += unique * 20
	if s.HTTPS { quality += 5 }
	if s.HasStructuredData { quality += 5 }
	quality += (1 - duplicate) * 10
	quality += (1 - boilerplate) * 5

	spam := 0.0
	spam += duplicate * 35
	spam += boilerplate * 20
	spam += clamp01((external-0.35)/0.65) * 20
	spam += clamp01((0.35-unique)/0.35) * 25

	return DocumentScore{Quality: roundScore(clamp100(quality)), Spam: roundScore(clamp100(spam))}
}

func contentDepthScore(runes int) float64 {
	switch {
	case runes <= 0:
		return 0
	case runes < 300:
		return float64(runes) / 600
	case runes < 1200:
		return 0.5 + float64(runes-300)/1800
	default:
		return 1
	}
}

func metadataScore(titleRunes, descriptionRunes int) float64 {
	title := 0.0
	if titleRunes >= 5 && titleRunes <= 120 { title = 1 }
	description := 0.0
	if descriptionRunes >= 20 && descriptionRunes <= 400 { description = 1 }
	return (title + description) / 2
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 { return 0 }
	if v > 1 { return 1 }
	return v
}

func clamp100(v float64) float64 {
	if math.IsNaN(v) || v < 0 { return 0 }
	if v > 100 { return 100 }
	return v
}

func roundScore(v float64) float64 {
	return math.Round(v*100) / 100
}
