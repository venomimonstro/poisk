# CURRENT SPRINT

**Sprint:** 28 — Security & Commercial Readiness Gate
**Status:** IN_PROGRESS — CODE/STATIC PASS, RUNTIME_GATE_PENDING

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
- Generic readiness, Quality, Capacity и Recovery evidence обязаны относиться к exact `RELEASE_VERSION + git commit + database schema` кандидата.
- Deployment-sensitive Browser/Security/Edge/MTA evidence обязаны быть собраны после preflight текущего candidate manifest.
- Re-stage изменённого manifest инвалидирует preflight; повторный неизменённый preflight/activate не сдвигает timestamp.
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
- [x] no unresolved **known code/static** P0/P1 security or data-integrity defects remain after Sprint 28 audit
- [x] public/backend proxy routes are explicit and cannot become generic internal API tunnels
- [x] auth cookies are HttpOnly/SameSite and Secure outside local/dev/test
- [x] browser mutations require CSRF; machine callbacks require bounded signed auth + replay protection
- [x] tenant isolation regression tests exist for Webmaster, Mail, Reviews, Billing/Claims critical paths
- [x] crawler/Webmaster external fetches revalidate DNS/IP after redirects and block private/metadata targets
- [x] attachment/blob paths cannot traverse storage root and downloads use safe response headers
- [x] Admin VIEWER/ANALYST cannot mutate; destructive OPERATOR/SUPERADMIN actions use preview/apply where required
- [x] secrets/private keys/raw credentials are absent from user-visible diagnostics and routine diagnostic responses audited in Sprint 28
- [x] readiness evidence metadata rejects password/token/secret/private-key style keys and remains size bounded
- [x] generic readiness PASS evidence is release/commit/schema-bound and checked for artifact reference + SHA-256 metadata by the final Gate
- [x] deployment-sensitive Browser/Security/Edge/MTA evidence predating current preflight is rejected
- [x] release manifest requires exact lowercase 40-character Git SHA; restage invalidates preflight
- [x] Owner Admin exposes the canonical readiness Gate read-only; it cannot set or bypass READY
- [x] migration versions are duplicate-protected; launch gate validates the exact repository/applied migration set
- [x] Quality history stores exact release/commit/schema binding plus golden/threshold SHA-256; old quality PASS cannot certify another candidate
- [x] Capacity benchmark stores exact release/commit/schema binding; HIGH bottlenecks block commercial readiness
- [x] Recovery PASS requires artifact metadata and exact release/commit/schema binding
- [x] browser/security/edge/MTA readiness evidence expires according to environment-sensitive TTL
- [ ] full build + unit tests have verified runtime evidence for the exact release candidate
- [ ] integration/security tests have verified runtime evidence against migrated PostgreSQL
- [ ] fresh install and previous-schema upgrade are verified for the exact release candidate
- [ ] browser smoke tests cover Search, Account, Webmaster, Maps/Reviews, Mail and Admin through the real HTTPS edge
- [ ] real Capacity `ISOLATED_1M` benchmark/snapshot + ADR for the exact release is available, fresh and contains no HIGH bottlenecks
- [ ] exact-release BACKUP and RESTORE drills are PASS, verifiable and fresh
- [ ] Internet Mail DNS/MTA/suppression/callback flow is verified when Internet Mail is enabled
- [x] incident/recovery/security runbook is updated
- [x] commercial-readiness report states exact blockers and does not convert missing evidence into PASS
- [x] immutable readiness evidence + fail-closed `readinessctl check` are implemented
- [x] no GitHub Actions / CI is added

## Current Launch Verdict

**COMMERCIAL READY: NO.**

Code/static gate is PASS, but runtime evidence is intentionally not inferred. The exact release candidate must record BUILD_UNIT, INTEGRATION, FRESH_INSTALL, UPGRADE, BROWSER_SMOKE, SECURITY_REGRESSION and EDGE_TLS_PROXY evidence for its exact `RELEASE_VERSION + READINESS_GIT_SHA + database schema`; deployment-sensitive evidence must be newer than the current preflight. Internet Mail additionally requires exact-candidate MTA_FLOW plus fresh DNS readiness for the configured domain/selector. Quality, Capacity and BACKUP/RESTORE must belong to the same exact candidate; Capacity must have no HIGH bottlenecks; Quality must carry golden/threshold hashes; recovery artifacts must have SHA-256 metadata, positive size and duration. Owner Admin `/admin/readiness` and `readinessctl check` use the same canonical Gate. Only that Gate returning `ready=true` changes this verdict.

## Forbidden Work
- new product verticals while this gate is open
- replacing the fixed stack
- Kubernetes/Kafka/RabbitMQ/Redis/Elasticsearch/OpenSearch/vector DB
- weakening auth/CSRF/SSRF controls to simplify testing
- exposing raw secrets, recipient lists, payment payloads or private keys to Admin UI
- marking the project commercial READY without actual runtime evidence
- GitHub Actions / CI
