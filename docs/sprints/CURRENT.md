# CURRENT SPRINT

**Sprint:** 20 — Data Hub
**Status:** IN_PROGRESS

## Goal
Построить Data Hub поверх уже существующих canonical Search/GEO/Webmaster/Demand данных: city/category pages, organization and website directories, privacy-safe trends, first-party aggregates and bounded programmatic SEO pages. Страницы должны быть детерминированными и строиться только из canonical данных; AI summaries допускаются только как вспомогательный слой и не могут быть единственным содержимым страницы.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–16 — PASS (code/static gate where noted in reports)
- Sprint 17 — PASS (code/static gate)
- Sprint 18 — PASS (code/static gate)
- Sprint 19 — PASS (code/static gate; report created)

## Allowed Work
- canonical city/category landing pages from organizations and address data
- organization directories and website/domain directories
- deterministic slugs and stable page identities
- privacy-bounded first-party trend aggregates from qualified Demand data
- directory filters and bounded pagination
- canonical/meta/robots/sitemap support for generated pages
- page-quality gates that prevent thin/duplicate programmatic pages
- deterministic statistics and related-page linking
- auxiliary generated summaries only when backed by canonical facts
- freshness/version metadata and incremental rebuilds
- tests, diagnostics and operator controls for Data Hub publication

## Forbidden Work
- Sprint 21 capacity benchmark/sharding decisions
- mass generation of thin pages without minimum data thresholds
- fabricated organizations, addresses, ratings, reviews or trends
- AI-generated facts not present in canonical data
- storing raw per-user search histories, IP or User-Agent for trends
- paid ranking or paid inclusion in organic directories
- separate Elasticsearch/OpenSearch index
- Redis/Kafka/RabbitMQ
- Kubernetes
- GitHub Actions/CI

## Definition of Done
- [ ] stable Data Hub page model and deterministic slug rules exist
- [ ] city/category pages are backed by canonical organizations/addresses only
- [ ] organization and website directories support bounded filters/pagination
- [ ] thin-page quality gate prevents publication below minimum evidence thresholds
- [ ] duplicate city/category combinations resolve to one canonical page identity
- [ ] trend signals use privacy-bounded aggregated Demand data only
- [ ] generated pages include canonical/meta/robots and sitemap-ready metadata
- [ ] related-page links are deterministic and bounded
- [ ] publication/version/freshness state supports incremental rebuild and rollback
- [ ] auxiliary summaries cannot introduce facts outside canonical aggregates
- [ ] no paid-plan/billing state changes organic directory ordering
- [ ] tests cover slug stability, thin-page suppression, pagination bounds and deterministic aggregates
- [ ] no Sprint 21 scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 20 report created
