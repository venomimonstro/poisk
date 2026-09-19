package extractor

import "testing"

func TestExactHashNormalizesCaseAndWhitespace(t *testing.T) {
	a := ExactHashHex("  Привет   МИР \n")
	b := ExactHashHex("привет мир")
	if a != b { t.Fatalf("hash mismatch: %s != %s", a, b) }
}

func TestSimHashNearDuplicate(t *testing.T) {
	a := SimHash64("поисковая система быстро находит полезные документы в интернете")
	b := SimHash64("поисковая система быстро находит полезные документы в интернете сегодня")
	d := HammingDistance64(a, b)
	if d <= 0 || d >= 32 { t.Fatalf("unexpected distance=%d", d) }
	if !IsNearDuplicate(a, b, d) { t.Fatalf("expected near duplicate at threshold=%d", d) }
	if IsNearDuplicate(a, b, d-1) { t.Fatalf("unexpected near duplicate below threshold=%d", d-1) }
}

func TestHammingDistanceKnownValues(t *testing.T) {
	if got := HammingDistance64(0, 0); got != 0 { t.Fatalf("distance=%d", got) }
	if got := HammingDistance64(0, ^uint64(0)); got != 64 { t.Fatalf("distance=%d", got) }
	if IsNearDuplicate(1, 2, -1) { t.Fatal("negative threshold must reject") }
}

func TestSimHashEmpty(t *testing.T) {
	if got := SimHash64("   "); got != 0 { t.Fatalf("simhash=%d", got) }
}
