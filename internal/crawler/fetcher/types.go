package fetcher

import (
	"errors"
	"net/http"
	"time"
)

var (
	ErrBodyTooLarge     = errors.New("response body exceeds hard limit")
	ErrTooManyRedirects = errors.New("too many redirects")
	ErrRedirectLocation = errors.New("invalid redirect location")
)

type Config struct {
	UserAgent             string
	MaxBodyBytes          int64
	MaxRedirects          int
	MaxRetries            int
	RequestTimeout        time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	RetryBaseDelay        time.Duration
	RetryMaxDelay         time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
}

func DefaultConfig() Config {
	return Config{
		UserAgent:             "PoiskBot/1.0",
		MaxBodyBytes:          8 << 20,
		MaxRedirects:          5,
		MaxRetries:            2,
		RequestTimeout:        20 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		RetryBaseDelay:        250 * time.Millisecond,
		RetryMaxDelay:         5 * time.Second,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   8,
		MaxConnsPerHost:       16,
	}
}

type Conditional struct {
	ETag         string
	LastModified string
}

type Result struct {
	StatusCode  int
	Header      http.Header
	Body        []byte
	FinalURL    string
	NotModified bool
	Attempts    int
	FetchedAt   time.Time
}
