package search

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestRerankUsesBoundedQualityAndSpamFactors(t *testing.T) {
	hits := []backend.Hit{
		{ID: 1, Score: 10, QualityScore: 0, SpamScore: 0},
		{ID: 2, Score: 9, QualityScore: 100, SpamScore: 0},
		{ID: 3, Score: 20, QualityScore: 0, SpamScore: 100},
	}
	got := rerank(hits)
	if got[0].ID != 2 || got[1].ID != 1 || got[2].ID != 3 { t.Fatalf("unexpected order: %+v", got) }
	if math.Abs(got[0].Score-10.8) > 1e-9 { t.Fatalf("quality adjusted score=%f", got[0].Score) }
	if math.Abs(got[2].Score-4.0) > 1e-9 { t.Fatalf("spam adjusted score=%f", got[2].Score) }
}

func TestRerankDoesNotMutateBackendHits(t *testing.T) {
	original := []backend.Hit{{ID: 1, Score: 10, QualityScore: 100}}
	_ = rerank(original)
	if original[0].Score != 10 { t.Fatalf("input mutated: %+v", original) }
}

type blockingBackend struct {
	calls     atomic.Int32
	active    atomic.Int32
	maxActive atomic.Int32
	gate      chan struct{}
}

func (b *blockingBackend) Search(ctx context.Context, q string, _ int) (backend.Result, error) {
	b.calls.Add(1)
	active := b.active.Add(1)
	for {
		old := b.maxActive.Load()
		if active <= old || b.maxActive.CompareAndSwap(old, active) { break }
	}
	defer b.active.Add(-1)
	select {
	case <-ctx.Done(): return backend.Result{}, ctx.Err()
	case <-b.gate:
		return backend.Result{Total: 1, Hits: []backend.Hit{{ID: 1, Score: 1, URL: "https://example.test", Host: "example.test", Title: q}}}, nil
	}
}

func TestIdenticalConcurrentQueriesAreCoalesced(t *testing.T) {
	b := &blockingBackend{gate: make(chan struct{})}
	s := &Service{Backend: b}
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := s.Search(context.Background(), Request{Query: "same query", Limit: 10})
			errs <- err
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(b.gate)
	wg.Wait()
	close(errs)
	for err := range errs { if err != nil { t.Fatal(err) } }
	if got := b.calls.Load(); got != 1 { t.Fatalf("backend calls=%d want=1", got) }
}

func TestBackendConcurrencyIsGloballyBounded(t *testing.T) {
	b := &blockingBackend{gate: make(chan struct{}, 4)}
	for i := 0; i < 4; i++ { b.gate <- struct{}{} }
	s := &Service{Backend: b, BackendConcurrency: make(chan struct{}, 1)}
	var wg sync.WaitGroup
	for _, q := range []string{"one", "two", "three", "four"} {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			_, _ = s.Search(context.Background(), Request{Query: query, Limit: 10})
		}(q)
	}
	wg.Wait()
	if got := b.maxActive.Load(); got > 1 { t.Fatalf("max concurrent backend calls=%d", got) }
}

func TestBackendConcurrencyWaitHonorsContext(t *testing.T) {
	b := &blockingBackend{gate: make(chan struct{})}
	s := &Service{Backend: b, BackendConcurrency: make(chan struct{}, 1)}
	s.BackendConcurrency <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := s.Search(ctx, Request{Query: "timeout", Limit: 10})
	<-s.BackendConcurrency
	if err == nil { t.Fatal("expected context deadline error") }
}
