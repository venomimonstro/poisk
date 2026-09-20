# CURRENT SPRINT

**Sprint:** 14 — GEO Search
**Status:** IN_PROGRESS

## Goal
Построить GEO-поиск поверх канонических OUR PLACE организаций: отдельный organizations index, city/category/nearby retrieval, географическое ранжирование, карточки результатов и clustering для карты. GEO должен быть изолирован от обычного Web Search и не менять его индекс/ранжирование.

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

## Allowed Work
- separate Manticore organizations index
- organization outbox consumer for ORGANIZATION events
- rebuildable organization index from PostgreSQL
- PostGIS/geospatial organization lookup
- city/category filters
- nearby/radius search
- lightweight GEO ranking
- GEO query intent/routing contract
- organization result cards
- map result clustering / bounded viewport queries
- GEO API and integration with the existing map/search surfaces
- latency/load limits using existing platform guard primitives
- unit/integration/security tests for coordinates, radius, filters and entity isolation

## Forbidden Work
- FIAS/GAR address index and full geocoding/autocomplete (Sprint 15)
- paid external map/place API required for core GEO search
- organization claiming (Sprint 17)
- billing/paid placement
- selling organic ranking positions
- merging GEO organizations directly into the web-document Manticore table
- Redis/Kafka/RabbitMQ
- GitHub Actions/CI

## Definition of Done
- [ ] ORGANIZATION outbox events are consumed by a dedicated organization indexer
- [ ] organization index is independently rebuildable from PostgreSQL
- [ ] stale organization versions cannot overwrite newer versions
- [ ] GEO search supports text + city/category filters
- [ ] nearby search supports bounded radius and coordinates
- [ ] invalid coordinates/radius are rejected before backend search
- [ ] GEO ranking combines text relevance, quality and bounded distance signal
- [ ] consumer organization cards expose stable place_id/name/category/address/coordinates
- [ ] viewport/map query is bounded and returns cluster-ready result data
- [ ] GEO query route is separate from ordinary WEB route
- [ ] ordinary web indexer does not consume ORGANIZATION events
- [ ] tests cover organization indexing, stale version, nearby bounds and filter isolation
- [ ] no Sprint 15 address/geocoding or paid ranking scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 14 report created
