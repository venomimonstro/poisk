package quality

import "math"

type RankedDoc struct {
	ID     int64
	Host   string
	Rating int
}

type Metrics struct {
	NDCG10      float64 `json:"ndcg_10"`
	MRR         float64 `json:"mrr"`
	Recall10    float64 `json:"recall_10"`
	ZeroResult  bool    `json:"zero_result"`
	Duplicate10 float64 `json:"duplicate_10"`
}

func Evaluate(ranked []RankedDoc, relevant map[int64]int) Metrics {
	return Metrics{
		NDCG10:      NDCGAt(ranked, relevant, 10),
		MRR:         MRR(ranked, relevant),
		Recall10:    RecallAt(ranked, relevant, 10),
		ZeroResult:  len(ranked) == 0,
		Duplicate10: DuplicateHostRateAt(ranked, 10),
	}
}

func NDCGAt(ranked []RankedDoc, relevant map[int64]int, k int) float64 {
	if k <= 0 || len(relevant) == 0 {
		return 0
	}
	limit := min(k, len(ranked))
	dcg := 0.0
	for i := 0; i < limit; i++ {
		rel := relevant[ranked[i].ID]
		if rel <= 0 {
			continue
		}
		dcg += gain(rel) / math.Log2(float64(i)+2)
	}
	ideal := make([]int, 0, len(relevant))
	for _, rel := range relevant {
		if rel > 0 {
			ideal = append(ideal, rel)
		}
	}
	sortDesc(ideal)
	if len(ideal) > k {
		ideal = ideal[:k]
	}
	idcg := 0.0
	for i, rel := range ideal {
		idcg += gain(rel) / math.Log2(float64(i)+2)
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func MRR(ranked []RankedDoc, relevant map[int64]int) float64 {
	for i, doc := range ranked {
		if relevant[doc.ID] > 0 {
			return 1 / float64(i+1)
		}
	}
	return 0
}

func RecallAt(ranked []RankedDoc, relevant map[int64]int, k int) float64 {
	if k <= 0 {
		return 0
	}
	totalRelevant := 0
	for _, rel := range relevant {
		if rel > 0 {
			totalRelevant++
		}
	}
	if totalRelevant == 0 {
		return 0
	}
	seen := make(map[int64]struct{}, k)
	found := 0
	limit := min(k, len(ranked))
	for i := 0; i < limit; i++ {
		id := ranked[i].ID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if relevant[id] > 0 {
			found++
		}
	}
	return float64(found) / float64(totalRelevant)
}

func DuplicateHostRateAt(ranked []RankedDoc, k int) float64 {
	if k <= 0 || len(ranked) == 0 {
		return 0
	}
	limit := min(k, len(ranked))
	counts := make(map[string]int, limit)
	duplicates := 0
	for i := 0; i < limit; i++ {
		host := ranked[i].Host
		if host == "" {
			continue
		}
		counts[host]++
		if counts[host] > 1 {
			duplicates++
		}
	}
	return float64(duplicates) / float64(limit)
}

func gain(rel int) float64 {
	return math.Pow(2, float64(rel)) - 1
}

func sortDesc(values []int) {
	for i := 1; i < len(values); i++ {
		v := values[i]
		j := i - 1
		for j >= 0 && values[j] < v {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = v
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
