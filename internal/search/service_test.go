package search

import (
	"context"
	"testing"

	"github.com/venomimonstro/poisk/internal/search/backend"
)

type fakeBackend struct {
	calls []string
}

func (f *fakeBackend) Search(_ context.Context, q string, _ int) (backend.Result, error) {
	f.calls = append(f.calls, q)
	if q == "ghbdtn" { return backend.Result{}, nil }
	if q == "привет" {
		return backend.Result{Total: 4, TookMS: 2, Hits: []backend.Hit{
			{ID: 1, Host: "a.test", URL: "https://a.test/1", Title: "1"},
			{ID: 2, Host: "a.test", URL: "https://a.test/2", Title: "2"},
			{ID: 3, Host: "a.test", URL: "https://a.test/3", Title: "3"},
			{ID: 4, Host: "b.test", URL: "https://b.test/1", Title: "4"},
		}}, nil
	}
	return backend.Result{}, nil
}

func TestVariantFallbackAndDomainDiversity(t *testing.T) {
	b := &fakeBackend{}
	s := &Service{Backend: b}
	resp, err := s.Search(context.Background(), Request{Query: "ghbdtn", Limit: 10})
	if err != nil { t.Fatal(err) }
	if resp.UsedQuery != "привет" { t.Fatalf("used=%q calls=%v", resp.UsedQuery, b.calls) }
	if len(resp.Results) != 3 { t.Fatalf("results=%+v", resp.Results) }
	countA := 0
	for _, result := range resp.Results { if result.Host == "a.test" { countA++ } }
	if countA != 2 { t.Fatalf("a.test count=%d", countA) }
}

func TestCacheAvoidsSecondBackendCall(t *testing.T) {
	b := &fakeBackend{}
	s := &Service{Backend: b, Cache: NewCache(8)}
	if _, err := s.Search(context.Background(), Request{Query: "привет", Limit: 10}); err != nil { t.Fatal(err) }
	firstCalls := len(b.calls)
	resp, err := s.Search(context.Background(), Request{Query: "привет", Limit: 10})
	if err != nil { t.Fatal(err) }
	if !resp.Cached { t.Fatal("expected cached response") }
	if len(b.calls) != firstCalls { t.Fatalf("backend calls before=%d after=%d", firstCalls, len(b.calls)) }
}
