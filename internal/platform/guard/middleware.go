package guard

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

type LatencyObserver interface {
	Observe(time.Duration)
}

type Middleware struct {
	Limiter     *Limiter
	Concurrency chan struct{}
	Deadline    time.Duration
	Latency     LatencyObserver
}

func NewMiddleware(limiter *Limiter, maxConcurrent int, deadline time.Duration) Middleware {
	if maxConcurrent <= 0 { maxConcurrent = 64 }
	if deadline <= 0 { deadline = 3 * time.Second }
	return Middleware{Limiter: limiter, Concurrency: make(chan struct{}, maxConcurrent), Deadline: deadline}
}

func (m Middleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		if m.Latency != nil { defer func() { m.Latency.Observe(time.Since(started)) }() }

		if m.Limiter != nil && !m.Limiter.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", "1")
			writeGuardError(w, http.StatusTooManyRequests, "rate_limited")
			return
		}

		if m.Concurrency == nil {
			writeGuardError(w, http.StatusServiceUnavailable, "overloaded")
			return
		}
		select {
		case m.Concurrency <- struct{}{}:
			defer func() { <-m.Concurrency }()
		default:
			writeGuardError(w, http.StatusServiceUnavailable, "overloaded")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), m.Deadline)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientKey(r *http.Request) string {
	host := strings.TrimSpace(r.RemoteAddr)
	if parsed, _, err := net.SplitHostPort(host); err == nil { host = parsed }
	if host == "" { return "unknown" }
	return host
}

func writeGuardError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
