package quality

import "testing"

func TestAuthorityScoreIsMonotonicAndBounded(t *testing.T) {
	low := AuthorityScore(1, 1)
	high := AuthorityScore(100, 1000)
	if low <= 0 || high <= low {
		t.Fatalf("low=%f high=%f", low, high)
	}
	if high > 100 {
		t.Fatalf("authority=%f", high)
	}
}

func TestAuthorityScoreRejectsEmptySignals(t *testing.T) {
	if got := AuthorityScore(0, 10); got != 0 { t.Fatalf("score=%f", got) }
	if got := AuthorityScore(10, 0); got != 0 { t.Fatalf("score=%f", got) }
}
