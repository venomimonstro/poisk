package metrics

import (
	"sort"
	"sync"
	"time"
)

type LatencySnapshot struct {
	Count int           `json:"count"`
	P50   time.Duration `json:"p50"`
	P95   time.Duration `json:"p95"`
	P99   time.Duration `json:"p99"`
}

type LatencyRecorder struct {
	mu      sync.Mutex
	samples []time.Duration
	next    int
	full    bool
}

func NewLatencyRecorder(capacity int) *LatencyRecorder {
	if capacity <= 0 { capacity = 2048 }
	return &LatencyRecorder{samples: make([]time.Duration, capacity)}
}

func (r *LatencyRecorder) Observe(d time.Duration) {
	if r == nil || len(r.samples) == 0 || d < 0 { return }
	r.mu.Lock()
	r.samples[r.next] = d
	r.next++
	if r.next >= len(r.samples) {
		r.next = 0
		r.full = true
	}
	r.mu.Unlock()
}

func (r *LatencyRecorder) Snapshot() LatencySnapshot {
	if r == nil { return LatencySnapshot{} }
	r.mu.Lock()
	count := r.next
	if r.full { count = len(r.samples) }
	values := make([]time.Duration, count)
	if r.full {
		copy(values, r.samples)
	} else {
		copy(values, r.samples[:count])
	}
	r.mu.Unlock()
	if len(values) == 0 { return LatencySnapshot{} }
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return LatencySnapshot{
		Count: len(values),
		P50: percentile(values, 0.50),
		P95: percentile(values, 0.95),
		P99: percentile(values, 0.99),
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 { return 0 }
	if p <= 0 { return sorted[0] }
	if p >= 1 { return sorted[len(sorted)-1] }
	idx := int(float64(len(sorted)-1)*p + 0.5)
	if idx < 0 { idx = 0 }
	if idx >= len(sorted) { idx = len(sorted)-1 }
	return sorted[idx]
}
