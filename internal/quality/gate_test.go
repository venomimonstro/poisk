package quality

import "testing"

func TestAggregateMetrics(t *testing.T) {
	got := AggregateMetrics([]Metrics{
		{NDCG10: 1, MRR: 1, Recall10: 0.5, Duplicate10: 0.2},
		{NDCG10: 0.5, MRR: 0.5, Recall10: 1, Duplicate10: 0.4, ZeroResult: true},
	})
	if got.Queries != 2 || got.NDCG10 != 0.75 || got.MRR != 0.75 || got.Recall10 != 0.75 || got.ZeroResultRate != 0.5 || got.Duplicate10 != 0.3 {
		t.Fatalf("aggregate=%+v", got)
	}
}

func TestGatePassAndFailure(t *testing.T) {
	thresholds := Thresholds{MinNDCG10: 0.7, MinMRR: 0.7, MinRecall10: 0.6, MaxZeroResult: 0.1, MaxDuplicate10: 0.3}
	pass := CheckGate(Aggregate{Queries: 10, NDCG10: 0.8, MRR: 0.9, Recall10: 0.8, ZeroResultRate: 0.05, Duplicate10: 0.2}, thresholds)
	if !pass.Pass || len(pass.Failures) != 0 {
		t.Fatalf("pass=%+v", pass)
	}
	fail := CheckGate(Aggregate{Queries: 10, NDCG10: 0.6, MRR: 0.9, Recall10: 0.8, ZeroResultRate: 0.2, Duplicate10: 0.2}, thresholds)
	if fail.Pass || len(fail.Failures) != 2 {
		t.Fatalf("fail=%+v", fail)
	}
}
