# CURRENT SPRINT

**Sprint:** 04 — HTTP Fetcher
**Status:** IN_PROGRESS

## Goal
Реализовать безопасный и ограниченный HTTP fetch layer поверх Sprint 03: connection reuse, redirect validation, deadlines, retries/backoff, conditional requests и response limits без утечек памяти/горутин.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS

## Allowed Work
- HTTP client/transport configuration
- connection pooling and keep-alive
- per-request deadlines/timeouts
- redirect handling with repeated SSRF validation
- ETag / If-None-Match
- Last-Modified / If-Modified-Since
- bounded response body reads
- Content-Length guards
- retry policy with exponential backoff/jitter
- Retry-After handling
- fetch result metadata
- unit/security/load-oriented tests

## Forbidden Work
- HTML/content extraction
- passages/dedup
- indexing/ranking/Search API
- GEO business logic
- Answer Engine
- JS rendering/browser automation

## Definition of Done
- [ ] HTTP transport has bounded connection pools and idle timeouts
- [ ] every initial and redirect target passes Sprint 03 SSRF validator
- [ ] redirect count is bounded
- [ ] request/header/body time budgets are bounded
- [ ] response body cannot exceed configured hard limit
- [ ] Content-Length oversized response is rejected early
- [ ] ETag and Last-Modified conditional requests supported
- [ ] 304 handled without content processing
- [ ] transient network/429/5xx failures use bounded retry/backoff
- [ ] Retry-After supported with maximum cap
- [ ] response bodies are always closed
- [ ] cancellation propagates through fetch/retry waits
- [ ] tests cover redirects, limits, conditional headers and retries
- [ ] no GitHub Actions/CI is added
- [ ] Sprint 04 report created
