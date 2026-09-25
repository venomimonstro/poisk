# CURRENT SPRINT

**Sprint:** 27 — Mail Deliverability Operations
**Status:** IN_PROGRESS

## Goal
Сделать Internet Mail эксплуатационно устойчивой после Sprint 26: подавлять заведомо недоставляемые адреса, безопасно управлять dead-letter/retry, ограничивать деградацию по доменам и обнаруживать DNS drift — без хранения raw bounce bodies и без добавления новых почтовых протоколов/инфраструктуры.

## Depends On
- Sprint 00–25 — PASS (code/static gate where noted in reports)
- Sprint 26 — PASS code/static gate; live MTA/DNS/browser/integration evidence remains external launch gate

## Architecture Boundary
- PostgreSQL остаётся source of truth для deliverability state, suppression, dead-letter и operational aggregates.
- Sprint 26 MTA boundary остаётся единственной публичной SMTP-границей; Go application по-прежнему не слушает public SMTP.
- Suppression относится только к точному canonical external recipient внутри tenant/user boundary и не превращается в глобальный пользовательский blacklist.
- Bounce diagnostics хранятся как bounded classification/code/metadata; raw DSN/bounce body не является canonical data.
- Backpressure и retry работают через существующий PostgreSQL queue, без Redis/Kafka/RabbitMQ.

## Allowed Work
- deterministic hard-bounce vs transient-failure classification from bounded MTA callback metadata
- per-user exact-recipient suppression with reason, counters, timestamps, expiry/manual clear
- suppression check before external enqueue/lease/submission
- bounded automatic suppression after repeated hard bounces
- safe dead-letter inspection/retry with tenant/operator authorization and no body exposure
- PostgreSQL-backed per-domain backpressure/cooldown based on aggregate failures
- bounded retry-after/domain cooldown integration in `mail-gateway-worker`
- periodic MX/SPF/DMARC/DKIM readiness snapshots and DNS drift status
- aggregate deliverability metrics: submitted/delivered/bounced/retry/dead/suppressed
- replay/event retention maintenance with explicit bounded windows
- idempotent reconciliation for stale `SUBMITTED` deliveries
- tests for suppression isolation/idempotency, hard/transient classification, cooldown bounds, retry safety and retention
- operator runbook/rollback documentation

## Forbidden Work
- IMAP/POP3 server/client implementation
- custom public SMTP server in the Go application
- bulk/marketing campaign sender, purchased lists or unsolicited-mail tooling
- wildcard forwarding, open relay or user-selectable MTA/SMTP credentials
- storing raw bounce bodies, SMTP credentials or DKIM private keys in PostgreSQL/source/browser
- global suppression keyed only by an external address across unrelated tenants
- Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch
- changing Search/GEO organic ranking
- GitHub Actions/CI
- Sprint 28+ scope

## Definition of Done
- [ ] deterministic bounded failure classification distinguishes hard bounce from retryable/transient failures
- [ ] repeated hard bounce can create a tenant-scoped exact-recipient suppression idempotently
- [ ] suppression has bounded reason/count/timestamps and expiry or explicit operator/user clear semantics
- [ ] suppressed recipients are rejected before new Internet delivery work reaches the MTA
- [ ] one tenant/user suppression cannot block another tenant/user
- [ ] dead-letter retry is explicit, auditable, idempotent and allowed only for retryable deliveries
- [ ] raw message bodies, raw DSNs and full recipient lists are not exposed by operational/admin health paths
- [ ] per-domain failure pressure can impose a bounded PostgreSQL-backed cooldown without permanent score mutation
- [ ] cooldown cannot be selected or bypassed by browser request parameters
- [ ] MX/SPF/DMARC/DKIM drift can be detected and persisted as bounded readiness snapshots
- [ ] stale replay/event records have documented bounded retention maintenance
- [ ] stale `SUBMITTED` deliveries have deterministic reconciliation behavior
- [ ] aggregate deliverability metrics are available without body inspection
- [ ] existing Sprint 25/26 internal and Internet Mail contracts remain backwards-compatible
- [ ] tests cover tenant isolation, idempotency, suppression threshold, transient failures, cooldown caps and retry authorization
- [ ] no new infrastructure or GitHub Actions/CI added
- [ ] Sprint 27 report created
