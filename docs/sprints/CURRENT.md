# CURRENT SPRINT

**Sprint:** 18 — Monetization
**Status:** IN_PROGRESS

## Goal
Добавить provider-neutral коммерческий контур поверх Webmaster/Growth/API без продажи organic ranking: планы Webmaster Pro, Agency, Site Search Pro, Business Pro, billing lifecycle и жёсткие usage limits. Денежные операции должны быть идемпотентными, суммы храниться целыми копейками, а отключение/истечение тарифа не должно повреждать canonical Search/Webmaster/GEO данные.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–16 — PASS (code/static gate where noted in reports)
- Sprint 17 — PASS (code/static gate)

## Allowed Work
- billing accounts tied to Webmaster users, agencies and claimed organizations
- immutable plan catalog with versioned prices/quotas
- Webmaster Pro, Agency, Site Search Pro and Business Pro entitlements
- provider-neutral invoices/payment events/subscription lifecycle
- idempotent payment-event ingestion
- integer kopeck accounting and immutable ledger entries
- monthly usage periods and atomic usage counters
- hard/soft quota checks for paid features
- plan downgrade/expiry/grace-period behavior
- Site Search usage limits and owner usage visibility
- Agency member/site limits
- Business Pro limits for claimed organizations
- Search/GEO API key + usage metering only where it does not change organic ranking
- billing/admin diagnostics, reconciliation and tests

## Forbidden Work
- paid organic ranking, paid crawl priority or SERP boosts
- advertising auction/ranking
- Sprint 19 Query Gap / Demand Driven Index implementation
- storing card data or payment credentials
- coupling core billing state to a single payment provider
- floating-point money
- Redis/Kafka/RabbitMQ
- Kubernetes
- Elasticsearch/OpenSearch
- GitHub Actions/CI

## Definition of Done
- [ ] every billable owner has an isolated billing account
- [ ] plan catalog is versioned and money is stored in integer kopecks
- [ ] subscription state transitions are explicit and auditable
- [ ] payment events are idempotent and cannot double-credit the ledger
- [ ] immutable ledger can reconcile invoices/payments/credits
- [ ] Webmaster Pro entitlements and limits are enforceable server-side
- [ ] Agency paid member/site limits are enforceable server-side
- [ ] Site Search Pro request/result quotas are enforceable server-side
- [ ] Business Pro entitlements require an active verified organization claim
- [ ] usage counters are atomic, period-bounded and owner isolated
- [ ] expiry/downgrade removes paid entitlements without deleting canonical data
- [ ] billing/admin diagnostics expose reconciliation state without payment secrets
- [ ] monetization does not alter organic ranking or crawl priority
- [ ] security/integration tests cover idempotency, isolation, quota race and downgrade
- [ ] no Sprint 19+ scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 18 report created
