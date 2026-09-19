package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
)

type TargetValidator interface {
	Validate(ctx context.Context, raw string) (crawlersecurity.ResolvedTarget, error)
}

type Fetcher struct {
	cfg       Config
	validator TargetValidator
	client    *http.Client
	sleep     func(context.Context, time.Duration) error
}

func New(cfg Config, validator TargetValidator) *Fetcher {
	cfg = normalizeConfig(cfg)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = true
	transport.MaxIdleConns = cfg.MaxIdleConns
	transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	transport.MaxConnsPerHost = cfg.MaxConnsPerHost
	transport.IdleConnTimeout = cfg.IdleConnTimeout
	transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	transport.TLSHandshakeTimeout = minDuration(10*time.Second, cfg.RequestTimeout)
	transport.ExpectContinueTimeout = time.Second
	transport.DialContext = validatedDialContext(validator, cfg.RequestTimeout)

	return &Fetcher{
		cfg:       cfg,
		validator: validator,
		client: &http.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		sleep: sleepContext,
	}
}

func (f *Fetcher) CloseIdleConnections() {
	if f != nil && f.client != nil {
		f.client.CloseIdleConnections()
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string, conditional Conditional) (Result, error) {
	if f == nil || f.validator == nil || f.client == nil {
		return Result{}, errors.New("fetcher is not initialized")
	}
	var last Result
	var lastErr error
	for attempt := 1; attempt <= f.cfg.MaxRetries+1; attempt++ {
		result, err := f.fetchOnce(ctx, rawURL, conditional)
		result.Attempts = attempt
		last, lastErr = result, err
		if err == nil && !isRetryableStatus(result.StatusCode) {
			return result, nil
		}
		if attempt > f.cfg.MaxRetries {
			break
		}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if err := f.sleep(ctx, retryDelay(f.cfg, attempt, result.Header)); err != nil {
			return Result{}, err
		}
	}
	if lastErr != nil {
		return last, lastErr
	}
	return last, nil
}

func (f *Fetcher) fetchOnce(ctx context.Context, rawURL string, conditional Conditional) (Result, error) {
	current := rawURL
	for redirects := 0; ; redirects++ {
		if redirects > f.cfg.MaxRedirects {
			return Result{}, ErrTooManyRedirects
		}
		target, err := f.validator.Validate(ctx, current)
		if err != nil {
			return Result{}, fmt.Errorf("validate target %q: %w", current, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL.String(), nil)
		if err != nil {
			return Result{}, err
		}
		req.Header.Set("User-Agent", f.cfg.UserAgent)
		req.Header.Set("Accept-Encoding", "identity")
		if conditional.ETag != "" {
			req.Header.Set("If-None-Match", conditional.ETag)
		}
		if conditional.LastModified != "" {
			req.Header.Set("If-Modified-Since", conditional.LastModified)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			return Result{}, err
		}
		if isRedirect(resp.StatusCode) {
			location := resp.Header.Get("Location")
			drainAndClose(resp.Body)
			if location == "" {
				return Result{}, ErrRedirectLocation
			}
			next, err := resolveRedirect(target.URL, location)
			if err != nil {
				return Result{}, fmt.Errorf("%w: %v", ErrRedirectLocation, err)
			}
			current = next
			continue
		}

		result := Result{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), FinalURL: target.URL.String(), FetchedAt: time.Now().UTC()}
		if resp.StatusCode == http.StatusNotModified {
			drainAndClose(resp.Body)
			result.NotModified = true
			return result, nil
		}
		if resp.ContentLength > f.cfg.MaxBodyBytes && resp.ContentLength >= 0 {
			drainAndClose(resp.Body)
			return result, ErrBodyTooLarge
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, f.cfg.MaxBodyBytes+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return result, readErr
		}
		if closeErr != nil {
			return result, closeErr
		}
		if int64(len(body)) > f.cfg.MaxBodyBytes {
			return result, ErrBodyTooLarge
		}
		result.Body = body
		return result, nil
	}
}

func normalizeConfig(cfg Config) Config {
	d := DefaultConfig()
	if cfg.UserAgent == "" { cfg.UserAgent = d.UserAgent }
	if cfg.MaxBodyBytes <= 0 { cfg.MaxBodyBytes = d.MaxBodyBytes }
	if cfg.MaxRedirects < 0 { cfg.MaxRedirects = d.MaxRedirects }
	if cfg.MaxRetries < 0 { cfg.MaxRetries = d.MaxRetries }
	if cfg.RequestTimeout <= 0 { cfg.RequestTimeout = d.RequestTimeout }
	if cfg.ResponseHeaderTimeout <= 0 { cfg.ResponseHeaderTimeout = d.ResponseHeaderTimeout }
	if cfg.IdleConnTimeout <= 0 { cfg.IdleConnTimeout = d.IdleConnTimeout }
	if cfg.RetryBaseDelay <= 0 { cfg.RetryBaseDelay = d.RetryBaseDelay }
	if cfg.RetryMaxDelay <= 0 { cfg.RetryMaxDelay = d.RetryMaxDelay }
	if cfg.MaxIdleConns <= 0 { cfg.MaxIdleConns = d.MaxIdleConns }
	if cfg.MaxIdleConnsPerHost <= 0 { cfg.MaxIdleConnsPerHost = d.MaxIdleConnsPerHost }
	if cfg.MaxConnsPerHost <= 0 { cfg.MaxConnsPerHost = d.MaxConnsPerHost }
	return cfg
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func isRetryableStatus(status int) bool {
	if status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout {
		return true
	}
	return status >= 500 && status <= 599
}

func resolveRedirect(base *url.URL, location string) (string, error) {
	next, err := url.Parse(location)
	if err != nil { return "", err }
	return base.ResolveReference(next).String(), nil
}

func retryDelay(cfg Config, attempt int, header http.Header) time.Duration {
	if header != nil {
		if d, ok := parseRetryAfter(header.Get("Retry-After"), time.Now()); ok {
			if d > cfg.RetryMaxDelay { return cfg.RetryMaxDelay }
			if d > 0 { return d }
		}
	}
	delay := cfg.RetryBaseDelay
	for i := 1; i < attempt; i++ {
		if delay >= cfg.RetryMaxDelay/2 { return cfg.RetryMaxDelay }
		delay *= 2
	}
	if delay > cfg.RetryMaxDelay { return cfg.RetryMaxDelay }
	return delay
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" { return 0, false }
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil { return 0, false }
	if when.Before(now) { return 0, true }
	return when.Sub(now), true
}

func drainAndClose(body io.ReadCloser) {
	if body == nil { return }
	_, _ = io.CopyN(io.Discard, body, 4<<10)
	_ = body.Close()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 { return nil }
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done(): return ctx.Err()
	case <-t.C: return nil
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b { return a }
	return b
}
