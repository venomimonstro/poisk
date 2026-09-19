# CURRENT SPRINT

**Sprint:** 03 — Scheduler + Crawler Security
**Status:** IN_PROGRESS

## Goal
Без скачивания контента реализовать безопасное планирование crawl: domain budgets, robots/sitemap discovery, URL normalization/trap guards и SSRF/DNS/IP validation.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS (GitHub Actions run `35456663094`)

## Allowed Work
- domain policy/budget repository
- global/domain scheduler primitives
- robots.txt parsing
- sitemap/sitemap-index parsing
- URL normalization
- URL trap/anomaly guards
- SSRF URL/DNS/IP validation
- sitemap hard limits
- tests/security tests

## Forbidden Work
- downloading normal page content
- content extraction
- ranking/search API
- GEO business logic
- Answer Engine
- JS rendering

## Definition of Done
- [ ] blocked domain никогда не планируется
- [ ] per-domain concurrency/RPS/budget доступны scheduler
- [ ] URL normalization deterministic
- [ ] private/loopback/link-local/metadata IP rejected
- [ ] redirect target может быть повторно валидирован тем же validator
- [ ] robots rules корректно применяются к PoiskBot
- [ ] Sitemap directives извлекаются из robots.txt
- [ ] sitemap XML и sitemap-index разбираются streaming-style с hard limits
- [ ] gzip/decompressed size limits определены
- [ ] recursive sitemap depth ограничен
- [ ] URL/path/query explosion guards реализованы
- [ ] unit/security tests PASS
- [ ] Sprint 03 report создан
