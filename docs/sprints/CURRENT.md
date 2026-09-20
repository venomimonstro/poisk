# CURRENT SPRINT

**Sprint:** 16 — Hardening
**Status:** IN_PROGRESS

## Goal
Довести Full MVP до управляемого production-state: защищённый Admin, RBAC/2FA, backup/restore, disk/memory watermarks, observability, release/rollback и security/recovery tests. Hardening не добавляет новые consumer-продукты — он делает уже существующие Search/Answer/Webmaster/Maps/GEO/Address контуры безопасно эксплуатируемыми.

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
- Sprint 11 — PASS (code/static gate)
- Sprint 12 — PASS (code/static gate)
- Sprint 13 — PASS (code/static gate)
- Sprint 14 — PASS (code/static gate)
- Sprint 15 — PASS (code/static gate)

## Allowed Work
- Admin authentication and server-side sessions
- RBAC with least-privilege roles
- TOTP 2FA and recovery-code lifecycle
- CSRF/session/cookie hardening for Admin mutations
- Admin audit log and mutation preview/dry-run requirements
- operational status pages for crawler/index/outbox/import backlogs
- disk/memory/load watermarks and fail-safe write/crawl shedding
- PostgreSQL/config backup commands and retention metadata
- restore command with explicit target/confirmation and restore verification
- rebuild documentation for disposable Manticore indexes
- release manifest/version metadata
- deployment preflight, release activation and rollback primitives
- observability endpoints/structured operational metrics
- security tests and recovery/restore test harness
- documentation/runbooks for incident, backup and rollback

## Forbidden Work
- Sprint 17 plugins, Site Search Widget, Agency flow or organization claiming
- Sprint 18 billing/paid plans/usage monetization
- paid organic ranking
- Redis/Kafka/RabbitMQ
- Kubernetes
- Elasticsearch/OpenSearch
- mandatory external monitoring SaaS
- GitHub Actions/CI

## Definition of Done
- [ ] Admin authentication is separate from public/Webmaster auth and defaults closed
- [ ] RBAC is enforced server-side for every Admin mutation
- [ ] TOTP 2FA + one-time recovery codes are supported
- [ ] Admin sessions are bounded/revocable and secrets are not stored in plaintext
- [ ] state-changing Admin requests have CSRF protection
- [ ] critical mutations require preview/dry-run and produce audit records
- [ ] operational status exposes bounded crawler/index/outbox/import health without sensitive data
- [ ] disk/memory watermarks can shed crawl/heavy writes before disk exhaustion/OOM
- [ ] PostgreSQL/config backup workflow is deterministic and documented
- [ ] restore workflow has explicit safety checks and verification
- [ ] Manticore web/organization/address rebuild paths are documented as disposable indexes
- [ ] release metadata identifies application/schema/map/index compatibility
- [ ] release preflight can block incompatible/broken activation
- [ ] rollback can return to previous release/config without data-destructive shortcuts
- [ ] security tests cover auth, RBAC, CSRF, session revocation and unsafe admin access
- [ ] recovery tests cover backup metadata, restore preflight and index rebuild contracts
- [ ] no Sprint 17+ scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 16 report created
