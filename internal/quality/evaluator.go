package quality

import (
	"context"
	"errors"
	"fmt"

	searchsvc "github.com/venomimonstro/poisk/internal/search"
)

type Searcher interface {
	Search(ctx context.Context, req searchsvc.Request) (searchsvc.Response, error)
}

type QueryReport struct {
	ID      string  `json:"id"`
	Query   string  `json:"query"`
	Metrics Metrics `json:"metrics"`
	Hits    int     `json:"hits"`
}

type Summary struct {
	JudgedQueries   int     `json:"judged_queries"`
	NDCG10          float64 `json:"ndcg_10"`
	MRR             float64 `json:"mrr"`
	Recall10        float64 `json:"recall_10"`
	ZeroResultRate  float64 `json:"zero_result_rate"`
	Duplicate10Rate float64 `json:"duplicate_10_rate"`
}

type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Summary       Summary       `json:"summary"`
	Queries       []QueryReport `json:"queries"`
}

func EvaluateGolden(ctx context.Context, searcher Searcher, set GoldenSet) (Report, error) {
	if searcher == nil { return Report{}, errors.New("quality searcher is nil") }
	if err := set.Validate(); err != nil { return Report{}, err }

	report := Report{SchemaVersion: 1, Queries: make([]QueryReport, 0, len(set.Queries))}
	for _, golden := range set.Queries {
		if len(golden.Judgments) == 0 { continue }
		response, err := searcher.Search(ctx, searchsvc.Request{Query: golden.Query, Limit: 20})
		if err != nil { return Report{}, fmt.Errorf("quality query %q: %w", golden.ID, err) }
		metrics, err := evaluateURLs(response.Results, golden.Judgments)
		if err != nil { return Report{}, fmt.Errorf("quality query %q: %w", golden.ID, err) }
		report.Queries = append(report.Queries, QueryReport{ID: golden.ID, Query: golden.Query, Metrics: metrics, Hits: len(response.Results)})
	}
	if len(report.Queries) == 0 { return report, nil }

	report.Summary.JudgedQueries = len(report.Queries)
	for _, query := range report.Queries {
		report.Summary.NDCG10 += query.Metrics.NDCG10
		report.Summary.MRR += query.Metrics.MRR
		report.Summary.Recall10 += query.Metrics.Recall10
		report.Summary.Duplicate10Rate += query.Metrics.Duplicate10
		if query.Metrics.ZeroResult { report.Summary.ZeroResultRate++ }
	}
	n := float64(len(report.Queries))
	report.Summary.NDCG10 /= n
	report.Summary.MRR /= n
	report.Summary.Recall10 /= n
	report.Summary.Duplicate10Rate /= n
	report.Summary.ZeroResultRate /= n
	return report, nil
}

func evaluateURLs(results []searchsvc.Result, judgments []Judgment) (Metrics, error) {
	urlIDs := make(map[string]int64, len(judgments))
	relevant := make(map[int64]int, len(judgments))
	for i, judgment := range judgments {
		normalized, err := NormalizeJudgmentURL(judgment.URL)
		if err != nil { return Metrics{}, err }
		id := int64(i + 1)
		urlIDs[normalized] = id
		relevant[id] = judgment.Grade
	}

	ranked := make([]RankedDoc, 0, len(results))
	unknownID := int64(-1)
	for _, result := range results {
		normalized, err := NormalizeJudgmentURL(result.URL)
		id := unknownID
		unknownID--
		if err == nil {
			if judgedID, ok := urlIDs[normalized]; ok { id = judgedID }
		}
		ranked = append(ranked, RankedDoc{ID: id, Host: result.Host})
	}
	return Evaluate(ranked, relevant), nil
}
