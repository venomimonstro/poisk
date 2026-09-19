package quality

import (
	"context"
	"testing"

	searchsvc "github.com/venomimonstro/poisk/internal/search"
)

type fakeQualitySearcher struct {
	responses map[string]searchsvc.Response
}

func (f fakeQualitySearcher) Search(_ context.Context, req searchsvc.Request) (searchsvc.Response, error) {
	return f.responses[req.Query], nil
}

func TestEvaluateGoldenProducesMachineReadableSummary(t *testing.T) {
	set := GoldenSet{Version: GoldenSchemaVersion, Queries: []GoldenQuery{
		{ID: "q1", Query: "alpha", Judgments: []Judgment{
			{URL: "https://a.test/1", Grade: 3},
			{URL: "https://b.test/1", Grade: 1},
		}},
	}}
	searcher := fakeQualitySearcher{responses: map[string]searchsvc.Response{
		"alpha": {Results: []searchsvc.Result{
			{URL: "https://a.test/1", Host: "a.test"},
			{URL: "https://x.test/1", Host: "x.test"},
			{URL: "https://b.test/1", Host: "b.test"},
		}},
	}}
	report, err := EvaluateGolden(context.Background(), searcher, set)
	if err != nil { t.Fatal(err) }
	if report.SchemaVersion != 1 || report.Summary.JudgedQueries != 1 || len(report.Queries) != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Summary.NDCG10 <= 0 || report.Summary.MRR != 1 || report.Summary.Recall10 != 1 {
		t.Fatalf("summary=%+v", report.Summary)
	}
}

func TestEvaluateGoldenSkipsUnjudgedSeedQueries(t *testing.T) {
	set := GoldenSet{Version: GoldenSchemaVersion, Queries: []GoldenQuery{{ID: "seed", Query: "query", Judgments: nil}}}
	report, err := EvaluateGolden(context.Background(), fakeQualitySearcher{}, set)
	if err != nil { t.Fatal(err) }
	if report.Summary.JudgedQueries != 0 || len(report.Queries) != 0 {
		t.Fatalf("report=%+v", report)
	}
}
