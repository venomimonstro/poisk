package guard

import (
	"testing"
	"time"
)

func TestLimiterBurstAndRefill(t *testing.T) {
	now := time.Unix(100, 0)
	l := NewLimiter(1, 2, 10, time.Minute)
	l.now = func() time.Time { return now }
	if !l.Allow("a") || !l.Allow("a") { t.Fatal("initial burst should pass") }
	if l.Allow("a") { t.Fatal("third request should be limited") }
	now = now.Add(time.Second)
	if !l.Allow("a") { t.Fatal("one token should refill") }
}

func TestLimiterClientCardinalityIsBounded(t *testing.T) {
	l := NewLimiter(1, 1, 3, time.Hour)
	now := time.Unix(100, 0)
	l.now = func() time.Time { return now }
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		if !l.Allow(key) { t.Fatalf("new client %s unexpectedly rejected", key) }
		now = now.Add(time.Second)
	}
	if got := l.Size(); got > 3 { t.Fatalf("size=%d", got) }
}

func TestLimiterEvictsIdleClients(t *testing.T) {
	now := time.Unix(100, 0)
	l := NewLimiter(1, 1, 10, time.Minute)
	l.now = func() time.Time { return now }
	_ = l.Allow("old")
	now = now.Add(2 * time.Minute)
	_ = l.Allow("new")
	if got := l.Size(); got != 1 { t.Fatalf("size=%d", got) }
}
