# CURRENT SPRINT

**Sprint:** 23 — Owner Admin Console
**Status:** IN_PROGRESS

## Goal
Превратить существующий защищённый Admin backend/operator UI в полноценный owner control plane, чтобы критические подсистемы можно было диагностировать и безопасно обслуживать без прямого доступа к PostgreSQL.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–21 — PASS (code/static gate where noted in reports)
- Sprint 22 — PASS code/static gate; runtime/browser/migration evidence remains an external launch gate
- commercial runtime/capacity evidence remains mandatory before launch

## Allowed Work
- owner dashboard and health/resource overview
- crawler domains/URLs/queue/history diagnostics
- index/outbox/rebuild diagnostics
- Search Quality/golden-query reports
- Demand/Query Gap diagnostics with permissioned suppress/reopen
- Answer Engine diagnostics
- spam/domain policy controls using preview/apply
- organization imports/review queue
- maps/address/data version visibility
- Webmaster users/sites/verification/support diagnostics
- consumer users/sessions/security events
- billing/invoice/reconciliation diagnostics
- Data Hub publication/suppression/rollback controls
- releases, backup/restore readiness and capacity snapshots
- RBAC viewer/analyst/operator/superadmin
- CSRF + preview/apply + immutable audit for critical mutations

## Forbidden Work
- direct unaudited destructive mutations
- exposing secrets, password hashes, raw session/auth/payment tokens
- bypassing tenant or admin RBAC boundaries
- paid influence on organic Search/Maps ranking
- Maps Reviews implementation (Sprint 24)
- Mail implementation (Sprint 25+)
- Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch
- GitHub Actions/CI

## Definition of Done
- [ ] SUPERADMIN can see health/resources/capacity and subsystem readiness from Admin
- [ ] crawler queue/history/domain diagnostics are available without DB access
- [ ] index/outbox/rebuild state is visible
- [ ] Search Quality, Query Gap and Answer diagnostics are visible
- [ ] organizations/imports/maps/address/data versions are visible
- [ ] Webmaster users/sites/verifications/support state are visible
- [ ] consumer users/sessions/security events are visible without sensitive token/hash fields
- [ ] billing subscriptions/invoices/payment reconciliation state is visible without payment secrets
- [ ] Data Hub/release/backup readiness is visible
- [ ] critical mutations require role + CSRF + preview/apply and immutable audit
- [ ] read-only roles cannot execute operator mutations
- [ ] no direct DB access is required for routine diagnosis
- [ ] tests cover RBAC and critical preview/apply boundaries
- [ ] no Sprint 24+ scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 23 report created
