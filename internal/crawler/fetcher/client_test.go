package fetcher

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
)

type fakeValidator struct {
	seen []string
	rejectHost string
}

func (v *fakeValidator) Validate(_ context.Context, raw string) (crawlersecurity.ResolvedTarget, error) {
	v.seen = append(v.seen, raw)
	u, err := url.Parse(raw)
	if err != nil { return crawlersecurity.ResolvedTarget{}, err }
	if u.Hostname() == v.rejectHost { return crawlersecurity.ResolvedTarget{}, errors.New("rejected target") }
	port := uint16(80)
	if u.Scheme == "https" { port = 443 }
	return crawlersecurity.ResolvedTarget{URL: u, Host: u.Hostname(), Port: port, IPs: []netip.Addr{netip.MustParseAddr("1.1.1.1")}}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)
func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func response(status int, headers http.Header, body string) *http.Response {
	if headers == nil { headers = make(http.Header) }
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

func testFetcher(v TargetValidator, fn roundTripFunc) *Fetcher {
	f := New(DefaultConfig(), v)
	f.client.Transport = fn
	f.client.Timeout = 0
	f.sleep = func(context.Context, time.Duration) error { return nil }
	return f
}

func TestConditionalHeadersAndNotModified(t *testing.T) {
	v := &fakeValidator{}
	f := testFetcher(v, func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("If-None-Match"); got != `"abc"` { t.Fatalf("If-None-Match=%q", got) }
		if got := r.Header.Get("If-Modified-Since"); got == "" { t.Fatal("missing If-Modified-Since") }
		return response(http.StatusNotModified, nil, ""), nil
	})
	got, err := f.Fetch(context.Background(), "https://example.com/a", Conditional{ETag: `"abc"`, LastModified: "Wed, 21 Oct 2015 07:28:00 GMT"})
	if err != nil { t.Fatal(err) }
	if !got.NotModified || len(got.Body) != 0 { t.Fatalf("unexpected result: %+v", got) }
}

func TestRedirectTargetIsValidatedBeforeSecondRequest(t *testing.T) {
	v := &fakeValidator{rejectHost: "blocked.example"}
	calls := 0
	f := testFetcher(v, func(r *http.Request) (*http.Response, error) {
		calls++
		h := make(http.Header)
		h.Set("Location", "https://blocked.example/private")
		return response(http.StatusFound, h, ""), nil
	})
	_, err := f.Fetch(context.Background(), "https://example.com/start", Conditional{})
	if err == nil { t.Fatal("expected redirect validation error") }
	if calls != 1 { t.Fatalf("network calls=%d, want 1", calls) }
	if len(v.seen) < 2 { t.Fatalf("validator calls=%v", v.seen) }
}

func TestBodyHardLimit(t *testing.T) {
	v := &fakeValidator{}
	cfg := DefaultConfig()
	cfg.MaxBodyBytes = 4
	f := New(cfg, v)
	f.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp := response(http.StatusOK, nil, "12345")
		resp.ContentLength = -1
		return resp, nil
	})
	f.client.Timeout = 0
	f.sleep = func(context.Context, time.Duration) error { return nil }
	_, err := f.Fetch(context.Background(), "https://example.com/large", Conditional{})
	if !errors.Is(err, ErrBodyTooLarge) { t.Fatalf("error=%v", err) }
}

func TestRetriesTransientStatus(t *testing.T) {
	v := &fakeValidator{}
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	calls := 0
	f := New(cfg, v)
	f.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 { return response(http.StatusServiceUnavailable, nil, "busy"), nil }
		return response(http.StatusOK, nil, "ok"), nil
	})
	f.client.Timeout = 0
	f.sleep = func(context.Context, time.Duration) error { return nil }
	got, err := f.Fetch(context.Background(), "https://example.com/retry", Conditional{})
	if err != nil { t.Fatal(err) }
	if got.StatusCode != http.StatusOK || got.Attempts != 3 || calls != 3 { t.Fatalf("result=%+v calls=%d", got, calls) }
}

func TestRetryAfterIsCapped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryMaxDelay = 3 * time.Second
	h := make(http.Header)
	h.Set("Retry-After", "120")
	if got := retryDelay(cfg, 1, h); got != 3*time.Second { t.Fatalf("delay=%s", got) }
}
