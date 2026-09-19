package guard

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	last     time.Time
	lastSeen time.Time
}

type Limiter struct {
	mu         sync.Mutex
	clients    map[string]*bucket
	rate       float64
	burst      float64
	maxClients int
	idleTTL    time.Duration
	now        func() time.Time
}

func NewLimiter(ratePerSecond float64, burst, maxClients int, idleTTL time.Duration) *Limiter {
	if ratePerSecond <= 0 { ratePerSecond = 5 }
	if burst <= 0 { burst = 10 }
	if maxClients <= 0 { maxClients = 10000 }
	if idleTTL <= 0 { idleTTL = 10 * time.Minute }
	return &Limiter{
		clients: make(map[string]*bucket, minInt(maxClients, 1024)),
		rate: ratePerSecond,
		burst: float64(burst),
		maxClients: maxClients,
		idleTTL: idleTTL,
		now: time.Now,
	}
}

func (l *Limiter) Allow(key string) bool {
	if l == nil || key == "" { return false }
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.clients[key]
	if !ok {
		l.evictLocked(now)
		if len(l.clients) >= l.maxClients { l.evictOldestLocked() }
		b = &bucket{tokens: l.burst, last: now, lastSeen: now}
		l.clients[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst { b.tokens = l.burst }
		b.last = now
	}
	b.lastSeen = now
	if b.tokens < 1 { return false }
	b.tokens--
	return true
}

func (l *Limiter) Size() int {
	if l == nil { return 0 }
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.clients)
}

func (l *Limiter) evictLocked(now time.Time) {
	cutoff := now.Add(-l.idleTTL)
	for key, b := range l.clients {
		if b.lastSeen.Before(cutoff) { delete(l.clients, key) }
	}
}

func (l *Limiter) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for key, b := range l.clients {
		if oldestKey == "" || b.lastSeen.Before(oldest) {
			oldestKey, oldest = key, b.lastSeen
		}
	}
	if oldestKey != "" { delete(l.clients, oldestKey) }
}

func minInt(a, b int) int { if a < b { return a }; return b }
