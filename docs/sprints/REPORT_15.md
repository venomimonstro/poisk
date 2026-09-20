# Sprint 15 — Address + Web ↔ GEO

Status: PASS (code/static gate)

## Implemented
- GAR/FIAS-compatible streaming XML decoder for `AS_ADDR_OBJ`, `AS_HOUSES`, hierarchy `ITEM` records.
- Bounded, resumable staging with checkpointing and a separate rejection ledger for malformed rows.
- Canonical `addresses` model with stable `(region_code, gar_object_id)` identity, versioning, parent hierarchy and PostGIS location.
- Resolver with bounded passes; inactive/history rows become `IGNORED`, unresolved active cycles/orphans become `ORPHAN` instead of partial canonical addresses.
- Transactional `ADDRESS` index outbox events.
- Isolated Manticore `addresses` table and dedicated `address-indexer` consumer.
- Stale address versions cannot overwrite newer versions.
- Independent `addressctl rebuild-index` from canonical PostgreSQL.
- `addressctl stage`, `finish`, `resolve`, `status`, `rebuild-index` operator flow with read-only import root and traversal protection.
- `/api/address/search` bounded prefix/exact search.
- `/api/address/geocode?id=` returns only canonical local coordinates and confidence metadata.
- `/api/address/reverse` uses local PostGIS only, radius <= 5 km and result count <= 20.
- Separate API route from WEB and GEO organization search, protected by the existing API guard.
- Docker Compose dedicated address indexer and read-only GAR import mount.
- Web ↔ GEO provenance table `organization_web_links`.
- Deterministic organization website-host matching and bounded Schema.org URL/phone confirmations via `orgctl match-web`.
- No paid geocoder, proprietary map scraper, Redis/Kafka/RabbitMQ, Elasticsearch/OpenSearch or CI added.

## Safety / integrity decisions
- External GAR rows never write directly to the searchable canonical table.
- Malformed rows do not require a fake OBJECTID; diagnostics are kept in `address_import_rejections`.
- Orphan/cycle rows never receive a fabricated parent/full address.
- Schema.org signals create provenance links only; they do not fuzzy-merge or silently overwrite canonical organizations.
- Reverse geocoding returns only coordinates already stored locally.

## Tests added
- GAR streaming/row parsing and resume/rejection paths.
- ADDRESS entity isolation and stale-version acknowledgement.
- Address query bounds and backend hard limits.
- Invalid coordinates rejected before reverse lookup.

## Gate limitation
The current execution container cannot resolve GitHub/dependency hosts, so a fresh `go test ./...`, frontend build and migration execution cannot be truthfully reported as run here. Sprint status is therefore code/static PASS. Runtime migration/build/test verification remains a deployment gate and is not represented as completed.

## Rollback / rebuild
- PostgreSQL is the canonical address source.
- Manticore address index is disposable and rebuilt with `app addressctl rebuild-index`.
- GAR import batches are resumable and retain rejection/orphan diagnostics.
