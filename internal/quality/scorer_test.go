package quality

import "testing"

func TestScoreDocumentRewardsUsefulContent(t *testing.T) {
	good := ScoreDocument(DocumentSignals{
		TextRunes: 3000, TitleRunes: 40, DescriptionRunes: 140,
		UniqueTokenRatio: 0.8, ExternalLinkRatio: 0.1, DuplicateRatio: 0.05,
		BoilerplateRatio: 0.1, HTTPS: true, HasStructuredData: true,
	})
	bad := ScoreDocument(DocumentSignals{
		TextRunes: 80, TitleRunes: 0, DescriptionRunes: 0,
		UniqueTokenRatio: 0.1, ExternalLinkRatio: 0.9, DuplicateRatio: 0.9,
		BoilerplateRatio: 0.8,
	})
	if good.Quality <= bad.Quality { t.Fatalf("good=%+v bad=%+v", good, bad) }
	if good.Spam >= bad.Spam { t.Fatalf("good=%+v bad=%+v", good, bad) }
}

func TestScoreDocumentIsBounded(t *testing.T) {
	got := ScoreDocument(DocumentSignals{
		TextRunes: 9999999, UniqueTokenRatio: 99, ExternalLinkRatio: 99,
		DuplicateRatio: 99, BoilerplateRatio: 99, HTTPS: true, HasStructuredData: true,
	})
	if got.Quality < 0 || got.Quality > 100 || got.Spam < 0 || got.Spam > 100 {
		t.Fatalf("score=%+v", got)
	}
}
