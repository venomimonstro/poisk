# CURRENT SPRINT

**Sprint:** 11 — Webmaster Free
**Status:** IN_PROGRESS

## Goal
Собрать бесплатный Webmaster-контур, который позволяет владельцу сайта зарегистрироваться, подтвердить владение доменом, добавить sitemap/URL, запросить удаление или переиндексацию, видеть индексный статус и базовую диагностику, а также получать агрегированные показы/клики/Answer citations.

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
- Sprint 10 — PASS (code/static gate)

## Allowed Work
- local account/session authentication for Webmaster
- password hashing and session lifecycle
- site registration and canonical host validation
- ownership verification tokens
- DNS TXT / HTML file / meta-tag verification contracts
- sitemap submission
- URL submit/reindex/delete requests
- index status and crawl diagnostics
- aggregate impressions/clicks/CTR
- Answer citation counters
- Webmaster API and basic UI
- audit events for sensitive Webmaster actions
- bounded validation/rate limits using existing platform primitives

## Forbidden Work
- paid Webmaster/Agency billing
- Maps/GEO/organizations
- Redis/Kafka/RabbitMQ
- external identity provider dependency required for core login
- arbitrary remote file fetching that bypasses crawler SSRF policy
- ranking manipulation controls sold to webmasters
- GitHub Actions/CI

## Definition of Done
- [ ] user can register/login/logout with bounded sessions
- [ ] passwords are stored only as strong hashes
- [ ] site can be added only as a canonical public HTTP(S) origin
- [ ] ownership verification is replay-safe and auditable
- [ ] verified owner can submit sitemap and individual URLs
- [ ] verified owner can request reindex/delete without directly mutating Manticore
- [ ] URL/index status and diagnostic reason are queryable
- [ ] impressions/clicks/CTR aggregation contract exists
- [ ] Answer citation aggregation contract exists
- [ ] one user cannot access another user's sites
- [ ] validation/security tests cover auth, ownership and cross-tenant isolation
- [ ] no GitHub Actions/CI added
- [ ] Sprint 11 report created
