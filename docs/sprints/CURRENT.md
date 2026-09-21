# CURRENT SPRINT

**Sprint:** 19 — Demand Driven Index
**Status:** IN_PROGRESS

## Goal
Добавить privacy-bounded Demand Driven Index поверх существующего Search/Crawler: считать Demand, Coverage, Quality, Freshness и Spam сигналы, материализовывать Query Gap только при подтверждённом спросе и слабой выдаче и давать crawler ограниченный feedback. Пользовательский запрос не должен напрямую создавать URL/crawl job, а платные продукты, referral и billing не должны влиять на organic crawl priority.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–16 — PASS (code/static gate where noted in reports)
- Sprint 17 — PASS (code/static gate)
- Sprint 18 — PASS (code/static gate; report created)

## Allowed Work
- normalized query hashing and bounded representative query retention only after gap qualification
- time-bucketed demand signals without IP/User-Agent storage
- Search Coverage Score from Demand/Coverage/Quality/Freshness/Spam
- Query Gap OPEN/WATCH/RESOLVED/SUPPRESSED lifecycle
- decay, caps and minimum independent time-bucket requirements
- manipulation protection against repeated/high-frequency query spam
- bounded feedback to existing canonical domains/URLs/category budgets
- bounded recrawl priority/frequency changes for trusted existing corpus only
- operator diagnostics and immutable feedback audit
- deterministic scorer/unit/integration tests and documentation

## Forbidden Work
- Sprint 20 city/category pages, trends, programmatic SEO or Data Hub
- direct query-to-URL crawl injection
- accepting arbitrary external URLs from query-gap signals
- paid ranking, paid crawl priority, billing/referral-based demand boosts
- storing IP addresses, User-Agent strings or per-user query histories for demand scoring
- unbounded raw query event logs
- Redis/Kafka/RabbitMQ
- Kubernetes
- Elasticsearch/OpenSearch
- GitHub Actions/CI

## Definition of Done
- [ ] demand signals are bucketed, capped and contain no IP/User-Agent identity
- [ ] repeated requests in one bucket cannot linearly inflate demand
- [ ] Query Gap requires minimum independent buckets plus low Coverage/Quality
- [ ] Search Coverage Score includes Demand, Coverage, Quality, Freshness and Spam
- [ ] scores are deterministic, bounded 0–100 and documented
- [ ] representative query text is retained only for qualified gaps and is length bounded
- [ ] Query Gap lifecycle supports OPEN/WATCH/RESOLVED/SUPPRESSED
- [ ] crawler feedback applies only to existing trusted canonical entities
- [ ] crawler priority/recrawl feedback is capped and expires/decays
- [ ] no query text can directly enqueue an arbitrary URL
- [ ] billing, referral and paid-plan state cannot affect demand/gap/crawl scores
- [ ] feedback actions are auditable and idempotent
- [ ] tests cover bucket anti-spam, score boundaries, gap qualification, feedback caps and paid-signal isolation
- [ ] no Sprint 20+ scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 19 report created
