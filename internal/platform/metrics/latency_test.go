package metrics

import (
	"testing"
	"time"
)

func TestLatencyRecorderPercentiles(t *testing.T) {
	r := NewLatencyRecorder(10)
	for i := 1; i <= 10; i++ { r.Observe(time.Duration(i) * time.Millisecond) }
	s := r.Snapshot()
	if s.Count != 10 { t.Fatalf("count=%d", s.Count) }
	if s.P50 != 6*time.Millisecond { t.Fatalf("p50=%s", s.P50) }
	if s.P95 != 10*time.Millisecond { t.Fatalf("p95=%s", s.P95) }
	if s.P99 != 10*time.Millisecond { t.Fatalf("p99=%s", s.P99) }
}

func TestLatencyRecorderIsBoundedRing(t *testing.T) {
	r := NewLatencyRecorder(3)
	for i := 1; i <= 6; i++ { r.Observe(time.Duration(i) * time.Millisecond) }
	s := r.Snapshot()
	if s.Count != 3 { t.Fatalf("count=%d", s.Count) }
	if s.P50 != 5*time.Millisecond { t.Fatalf("p50=%s", s.P50) }
	if s.P99 != 6*time.Millisecond { t.Fatalf("p99=%s", s.P99) }
}
