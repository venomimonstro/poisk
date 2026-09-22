# CURRENT SPRINT

**Sprint:** 22 — Unified Identity + Webmaster Cabinet
**Status:** IN_PROGRESS

## Goal
Создать единый consumer account/session слой для публичных сервисов Poisk, сохранив Admin как отдельную security boundary, и реализовать полноценный self-service кабинет Webmaster: регистрация/вход, добавление сайта, понятная установка verification proof, проверка владения, sitemap/URL operations, индекс/диагностика и first-party аналитика.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–21 — PASS (code/static gate where noted in reports)
- commercial runtime/capacity evidence remains an external launch gate and is not considered satisfied by code/static reports

## Allowed Work
- canonical consumer user identity and compatibility link from existing Webmaster users
- HttpOnly/Secure/SameSite sessions, CSRF, email verification/reset token primitives
- session list/revoke and account security events
- keep `admin_users` fully separate from consumer identities
- migrate/link Webmaster ownership, agency membership, organization claims and billing ownership without data loss
- public service navigation/account shell
- `/webmaster` login/registration/onboarding UI
- add/normalize a site and show exact DNS TXT / HTML file / META verification instructions
- check/reissue ownership proof and verification status
- verified-site dashboard
- sitemap add/status/error UI
- URL Submit/Reindex/Delete UI
- index/crawl diagnostics, robots/canonical/404/500 views
- impressions/clicks/CTR/Answer citation analytics with bounded date ranges
- Webmaster Pro entitlement/usage visibility without ranking influence
- tenant-isolation, CSRF, session and verification tests

## Forbidden Work
- merge Admin auth into consumer auth
- Maps reviews implementation (Sprint 24)
- Mail implementation (Sprint 25+)
- paid organic ranking/crawl priority
- arbitrary external URLs outside verified Webmaster ownership
- storing raw passwords/session tokens/reset tokens
- Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch
- GitHub Actions/CI

## Definition of Done
- [ ] canonical consumer user/session schema exists with compatibility mapping from Webmaster users
- [ ] Admin remains an isolated authentication realm
- [ ] session tokens are hashed, bounded, revocable and cookie-safe
- [ ] CSRF protects cookie-authenticated mutations
- [ ] existing Webmaster sites/metrics/billing ownership are preserved
- [ ] new user can register/login and reach `/webmaster`
- [ ] user can add a site and receive exact ownership instructions
- [ ] DNS TXT / HTML file / META verification flows are usable from UI
- [ ] successful verification unlocks sitemap and URL operations
- [ ] Webmaster UI exposes index/crawl diagnostics and bounded analytics
- [ ] ownership/tenant isolation is enforced server-side, not only in UI
- [ ] service navigation links Search / Maps / Webmaster / account; Mail may appear only when implemented
- [ ] tests cover session isolation, CSRF, ownership verification and cross-tenant access
- [ ] no Sprint 24+ scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 22 report created
