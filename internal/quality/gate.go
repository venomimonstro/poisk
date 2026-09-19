package quality

import "fmt"

type Thresholds struct {
	MinQueries       int     `json:"min_queries"`
	MinNDCG10        float64 `json:"min_ndcg_10"`
	MinMRR           float64 `json:"min_mrr"`
	MinRecall10      float64 `json:"min_recall_10"`
	MaxZeroResult    float64 `json:"max_zero_result"`
	MaxDuplicate10   float64 `json:"max_duplicate_10"`
}

type Aggregate struct {
	Queries        int     `json:"queries"`
	NDCG10         float64 `json:"ndcg_10"`
	MRR            float64 `json:"mrr"`
	Recall10       float64 `json:"recall_10"`
	ZeroResultRate float64 `json:"zero_result_rate"`
	Duplicate10    float64 `json:"duplicate_10"`
}

type GateResult struct {
	Pass     bool     `json:"pass"`
	Failures []string `json:"failures,omitempty"`
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		MinQueries:       50,
		MinNDCG10:        0.55,
		MinMRR:           0.60,
		MinRecall10:      0.55,
		MaxZeroResult:    0.12,
		MaxDuplicate10:   0.20,
	}
}

func CheckGate(metrics Aggregate, thresholds Thresholds) GateResult {
	failures := make([]string, 0, 6)
	minQueries := thresholds.MinQueries
	if minQueries <= 0 { minQueries = 1 }
	if metrics.Queries < minQueries {
		failures = append(failures, fmt.Sprintf("queries %d < %d", metrics.Queries, minQueries))
	}
	if metrics.NDCG10 < thresholds.MinNDCG10 {
		failures = append(failures, fmt.Sprintf("ndcg_10 %.6f < %.6f", metrics.NDCG10, thresholds.MinNDCG10))
	}
	if metrics.MRR < thresholds.MinMRR {
		failures = append(failures, fmt.Sprintf("mrr %.6f < %.6f", metrics.MRR, thresholds.MinMRR))
	}
	if metrics.Recall10 < thresholds.MinRecall10 {
		failures = append(failures, fmt.Sprintf("recall_10 %.6f < %.6f", metrics.Recall10, thresholds.MinRecall10))
	}
	if metrics.ZeroResultRate > thresholds.MaxZeroResult {
		failures = append(failures, fmt.Sprintf("zero_result_rate %.6f > %.6f", metrics.ZeroResultRate, thresholds.MaxZeroResult))
	}
	if metrics.Duplicate10 > thresholds.MaxDuplicate10 {
		failures = append(failures, fmt.Sprintf("duplicate_10 %.6f > %.6f", metrics.Duplicate10, thresholds.MaxDuplicate10))
	}
	return GateResult{Pass: len(failures) == 0, Failures: failures}
}

func AggregateMetrics(items []Metrics) Aggregate {
	if len(items) == 0 { return Aggregate{} }
	var out Aggregate
	out.Queries = len(items)
	for _, item := range items {
		out.NDCG10 += item.NDCG10
		out.MRR += item.MRR
		out.Recall10 += item.Recall10
		out.Duplicate10 += item.Duplicate10
		if item.ZeroResult { out.ZeroResultRate++ }
	}
	n := float64(len(items))
	out.NDCG10 /= n
	out.MRR /= n
	out.Recall10 /= n
	out.ZeroResultRate /= n
	out.Duplicate10 /= n
	return out
}
