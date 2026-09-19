package quality

import (
	"math"
	"testing"
)

func TestPerfectRankingHasNDCGOne(t *testing.T) {
	ranked := []RankedDoc{{ID: 1}, {ID: 2}, {ID: 3}}
	relevant := map[int64]int{1: 3, 2: 2, 3: 1}
	got := NDCGAt(ranked, relevant, 10)
	if math.Abs(got-1) > 1e-12 {
		t.Fatalf("ndcg=%f", got)
	}
}

func TestMRRUsesFirstRelevantDocument(t *testing.T) {
	ranked := []RankedDoc{{ID: 9}, {ID: 8}, {ID: 7}}
	relevant := map[int64]int{8: 1, 7: 2}
	if got := MRR(ranked, relevant); got != 0.5 {
		t.Fatalf("mrr=%f", got)
	}
}

func TestRecallIgnoresDuplicateDocumentIDs(t *testing.T) {
	ranked := []RankedDoc{{ID: 1}, {ID: 1}, {ID: 2}}
	relevant := map[int64]int{1: 1, 2: 1, 3: 1}
	got := RecallAt(ranked, relevant, 3)
	want := 2.0 / 3.0
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("recall=%f want=%f", got, want)
	}
}

func TestDuplicateHostRate(t *testing.T) {
	ranked := []RankedDoc{{Host: "a.test"}, {Host: "a.test"}, {Host: "b.test"}, {Host: "a.test"}}
	got := DuplicateHostRateAt(ranked, 10)
	if math.Abs(got-0.5) > 1e-12 {
		t.Fatalf("duplicate rate=%f", got)
	}
}

func TestEvaluateZeroResult(t *testing.T) {
	got := Evaluate(nil, map[int64]int{1: 1})
	if !got.ZeroResult || got.NDCG10 != 0 || got.MRR != 0 || got.Recall10 != 0 {
		t.Fatalf("metrics=%+v", got)
	}
}
