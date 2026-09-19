# CURRENT SPRINT

**Sprint:** 10 — Performance / Load Protection
**Status:** IN_PROGRESS

## Goal
Защитить Search/Answer API от перегрузки и дорогих запросов: end-to-end deadlines, bounded concurrency, single-flight для одинаковых запросов, rate limiting, load shedding, query complexity limits и измеримые latency/load primitives без новых инфраструктурных сервисов.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS
- Sprint 04 — PASS
- Sprint 05 — PASS
- Sprint 06 — PASS
- Sprint 07 — PASS
- Sprint 08 — PASS
- Sprint 09 — PASS (code/static gate)

## Allowed Work
- API request deadlines
- bounded concurrency semaphores
- local token-bucket/rate limiting
- single-flight/coalescing for identical searches
- query complexity scoring/limits
- load shedding and 429/503 behavior
- latency instrumentation primitives
- cache hardening
- bounded benchmarks/tests
- progressive Search/Answer frontend behavior

## Forbidden Work
- Redis/Kafka/RabbitMQ
- distributed rate-limit infrastructure
- Kubernetes/microservices
- LLM on every query
- GEO/maps/organizations
- vector DB/embeddings

## Definition of Done
- [ ] Search and Answer have explicit request deadlines
- [ ] concurrent backend searches are globally bounded
- [ ] identical concurrent Search requests are coalesced
- [ ] per-client API rate limit is bounded in memory
- [ ] rate limiter has bounded client-state cardinality/eviction
- [ ] query complexity guard rejects pathological input before backend
- [ ] overload returns deterministic 429 or 503 instead of queue explosion
- [ ] local cache remains bounded under many unique queries
- [ ] latency counters/histogram primitives available for P50/P95/P99 reporting
- [ ] tests cover deadline, rate, overload, single-flight and complexity limits
- [ ] no GitHub Actions/CI added
- [ ] Sprint 10 report created
