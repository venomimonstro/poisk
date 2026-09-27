# CURRENT SPRINT

**Sprint:** 28 — Security & Commercial Readiness Gate
**Status:** IN_PROGRESS

## Goal
Провести финальный логический, security и operational аудит всего продукта перед коммерческим использованием. Исправить P0/P1 проблемы, доказать tenant isolation, auth/session/CSRF/SSRF/file/mail boundaries, migration safety, recovery/capacity readiness и сформировать честный launch verdict на основании runtime evidence, а не только наличия кода.

## Depends On
- Sprint 00–26 — PASS (code/static gate where noted in reports)
- Sprint 27 — PASS code/static gate; runtime/integration/DNS/MTA evidence remains external launch gate

## Architecture Boundary
- Текущий stack, modular monolith и PostgreSQL source-of-truth сохраняются.
- Новые product verticals, ranking factors, queue systems и infrastructure platforms запрещены.
- Security fixes используют существующие auth/RBAC/preview/apply/audit/guard patterns.
- Commercial READY разрешён только при подтверждённых build/test/migration/recovery/capacity/browser/MTA evidence.
- Code/static PASS не равен production PASS.

## Workstreams
1. Public/API/Next proxy attack-surface audit.
2. Auth/session/CSRF/RBAC/tenant isolation audit.
3. SSRF/injection/path traversal/upload/download/browser security audit.
4. Secrets/privacy/logging/retention audit.
5. Migration/backup/restore/recovery audit.
6. Capacity/load-shedding/failure-isolation audit.
7. Browser smoke matrix and commercial launch checklist.

## Definition of Done
- [ ] no unresolved known P0/P1 security or data-integrity defects
- [ ] public/backend proxy routes are explicit and cannot become generic internal API tunnels
- [ ] auth cookies are HttpOnly/SameSite and Secure outside local/dev/test
- [ ] browser mutations require CSRF; machine callbacks require bounded signed auth + replay protection
- [ ] tenant isolation regression tests cover Webmaster, Mail, Reviews, Billing/Claims critical paths
- [ ] crawler/Webmaster external fetches revalidate DNS/IP after redirects and block private/metadata targets
- [ ] attachment/blob paths cannot traverse storage root and downloads use safe response headers
- [ ] Admin VIEWER/ANALYST cannot mutate; destructive OPERATOR/SUPERADMIN actions use preview/apply where required
- [ ] secrets/private keys/raw credentials are absent from user-visible diagnostics and routine logs
- [ ] migration versions are unique and upgrade-safe
- [ ] full build + unit tests have verified runtime evidence
- [ ] integration tests have verified runtime evidence against migrated PostgreSQL
- [ ] fresh install and previous-schema upgrade are verified
- [ ] browser smoke tests cover Search, Account, Webmaster, Maps/Reviews, Mail and Admin
- [ ] real Capacity benchmark/snapshot is available
- [ ] current-schema BACKUP and RESTORE drills are PASS
- [ ] Internet Mail DNS/MTA/suppression/callback flow is verified when Internet Mail is enabled
- [ ] incident/recovery/security runbook is updated
- [ ] final commercial-readiness report states exact blockers and does not convert missing evidence into PASS
- [ ] no GitHub Actions / CI is added

## Forbidden Work
- new product verticals while this gate is open
- replacing the fixed stack
- Kubernetes/Kafka/RabbitMQ/Redis/Elasticsearch/OpenSearch/vector DB
- weakening auth/CSRF/SSRF controls to simplify testing
- exposing raw secrets, recipient lists, payment payloads or private keys to Admin UI
- marking the project commercial READY without actual runtime evidence
- GitHub Actions / CI
