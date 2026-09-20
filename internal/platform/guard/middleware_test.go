package guard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type latencySpy struct{ count int }
func (s *latencySpy) Observe(time.Duration) { s.count++ }

func TestProtectReturns429WhenRateLimited(t *testing.T) {
	limiter := NewLimiter(0.0001, 1, 10, time.Hour)
	m := NewMiddleware(limiter, 1, time.Second)
	h := m.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		h.ServeHTTP(rr, req)
		if rr.Code != want { t.Fatalf("request %d status=%d want=%d", i, rr.Code, want) }
	}
}

func TestProtectReturns503WhenConcurrencyFull(t *testing.T) {
	m := NewMiddleware(nil, 1, time.Second)
	m.Concurrency <- struct{}{}
	h := m.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil))
	<-m.Concurrency
	if rr.Code != http.StatusServiceUnavailable { t.Fatalf("status=%d", rr.Code) }
}

func TestProtectAddsRequestDeadline(t *testing.T) {
	m := NewMiddleware(nil, 1, 5*time.Millisecond)
	h := m.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			if r.Context().Err() != context.DeadlineExceeded { t.Errorf("err=%v", r.Context().Err()) }
			w.WriteHeader(http.StatusGatewayTimeout)
		case <-time.After(100 * time.Millisecond):
			t.Error("deadline did not fire")
		}
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil))
	if rr.Code != http.StatusGatewayTimeout { t.Fatalf("status=%d", rr.Code) }
}

func TestProtectRecordsLatencyForRejectedAndAcceptedRequests(t *testing.T) {
	spy := &latencySpy{}
	m := NewMiddleware(nil, 1, time.Second)
	m.Latency = spy
	h := m.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil))
	if spy.count != 1 { t.Fatalf("observations=%d", spy.count) }
}
