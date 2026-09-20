# CURRENT SPRINT

**Sprint:** 15 — Address Search & Geocoding
**Status:** IN_PROGRESS

## Goal
Построить собственный адресный слой без обязательной зависимости от платных картографических API: импорт адресного справочника в staging, канонический Address Index, нормализация и иерархия адреса, быстрый autocomplete/search и точный переход ADDRESS → координаты/карта. Адресный индекс должен быть rebuildable и изолирован от Web/GEO organizations index.

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

## Allowed Work
- FIAS/GAR-compatible source adapter contracts
- bounded/resumable address import staging
- canonical address objects with stable source identity/version
- parent/child address hierarchy and normalized display path
- address type/level normalization
- explicit source coordinates when available
- isolated Manticore address index
- ADDRESS outbox consumer and independent rebuild
- address autocomplete/prefix search with strict limits
- exact/fuzzy-light address search without heavy ML
- forward geocoding from indexed address to canonical coordinates
- reverse lookup only from locally indexed canonical coordinates
- ADDRESS route contract separate from WEB/GEO
- map handoff to canonical address coordinates
- tests for hierarchy, normalization, duplicate imports, stale versions and query bounds

## Forbidden Work
- mandatory paid geocoding/map APIs
- scraping proprietary map providers
- unbounded full GAR file loading into RAM
- direct source writes into searchable canonical tables
- organization claiming/billing/paid placement
- selling organic ranking positions
- Redis/Kafka/RabbitMQ
- Elasticsearch/OpenSearch
- GitHub Actions/CI

## Definition of Done
- [ ] address source rows are staged before canonical mutation
- [ ] import is bounded, resumable and idempotent
- [ ] canonical address identity/version and hierarchy exist
- [ ] malformed or orphaned rows fail safely with diagnostics
- [ ] duplicate source rows cannot create duplicate canonical addresses
- [ ] address search index is separate from WEB and organizations indexes
- [ ] ADDRESS outbox consumer is entity-isolated and stale-version safe
- [ ] address index can be rebuilt from PostgreSQL
- [ ] autocomplete returns bounded prefix results
- [ ] address search supports normalized exact/phrase lookup
- [ ] forward geocoding returns only canonical/local coordinates with confidence metadata
- [ ] reverse lookup is bounded by radius/result count
- [ ] address/map handoff uses stable address_id and coordinates
- [ ] tests cover hierarchy, import resume, stale index versions and malformed queries
- [ ] no paid external geocoder is required for core functionality
- [ ] no Sprint 16+ commercial/claiming scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 15 report created
