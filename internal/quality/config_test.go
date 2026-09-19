package quality

import (
	"strings"
	"testing"
)

func TestLoadThresholds(t *testing.T) {
	got, err := LoadThresholds(strings.NewReader(`{"min_ndcg_10":0.65,"min_mrr":0.60,"min_recall_10":0.50,"max_zero_result":0.20,"max_duplicate_10":0.35}`))
	if err != nil { t.Fatal(err) }
	if got.MinNDCG10 != 0.65 || got.MaxZeroResult != 0.20 { t.Fatalf("thresholds=%+v", got) }
}

func TestLoadThresholdsRejectsOutOfRangeAndUnknownFields(t *testing.T) {
	if _, err := LoadThresholds(strings.NewReader(`{"min_ndcg_10":1.1}`)); err == nil {
		t.Fatal("expected range error")
	}
	if _, err := LoadThresholds(strings.NewReader(`{"min_ndcg_10":0.5,"extra":1}`)); err == nil {
		t.Fatal("expected unknown field error")
	}
}
