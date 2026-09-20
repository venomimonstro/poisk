# CURRENT SPRINT

**Sprint:** 12 — Maps
**Status:** IN_PROGRESS

## Goal
Добавить независимый картографический слой на базе OSM/PMTiles/MapLibre: versioned map artifacts, безопасный manifest, runtime map configuration, frontend map rendering и rollback без зависимости от GEO/organizations следующих спринтов.

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

## Allowed Work
- OSM attribution and map source policy
- PMTiles artifact manifest and immutable versions
- active map version pointer
- rollback to previous validated version
- map file integrity metadata (size/hash/version)
- bounded map manifest API
- MapLibre frontend integration
- PMTiles protocol integration
- simple map screen and viewport state
- static/self-hosted map style configuration
- validation tests for manifests/version switching/rollback
- deployment paths for immutable map artifacts

## Forbidden Work
- organization import
- GEO search or nearby ranking
- business cards/claiming
- FIAS/GAR geocoding
- paid map APIs required for core rendering
- arbitrary remote tile proxying
- runtime mutation of immutable PMTiles files
- Redis/Kafka/RabbitMQ
- GitHub Actions/CI

## Definition of Done
- [ ] map artifacts are immutable and version identified
- [ ] active version is stored separately from artifacts
- [ ] manifest validates path/hash/size/bounds/version
- [ ] map config API exposes only validated active artifact metadata
- [ ] rollback can atomically switch to a prior validated version
- [ ] MapLibre renders the configured PMTiles source
- [ ] OSM attribution is visible
- [ ] missing/corrupt active version fails safely
- [ ] tests cover manifest validation, activation and rollback rules
- [ ] no GEO/organization scope is pulled into Sprint 12
- [ ] no GitHub Actions/CI added
- [ ] Sprint 12 report created
