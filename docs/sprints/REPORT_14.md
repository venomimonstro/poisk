# Sprint 14 — GEO Search Report

**Status:** PASS (code/static gate)

## Delivered
- Dedicated Manticore `organizations` index isolated from `web_documents`.
- Dedicated `organization-indexer` runtime consuming only `ORGANIZATION` outbox events.
- Existing `organizations-worker` preserved for Sprint 13 canonical APPLY; import processing and search indexing are separate processes.
- Canonical `city_key` and generated PostGIS geography location added via migration `000011_geo_search.sql`.
- Organization import path carries `city_key` end-to-end: JSONL → staging → canonical OUR PLACE → organization index.
- Version-safe organization indexing: stale outbox versions cannot overwrite newer canonical/index state.
- `orgctl rebuild-index` rebuilds only the organizations Manticore index from canonical PostgreSQL data.
- `/api/geo/search` supports bounded text, city, category and nearby/radius retrieval.
- Coordinates and radius are validated before backend access; radius is capped at 100 km.
- GEO ranking combines backend text relevance, canonical quality, source count and bounded distance penalty.
- `/api/geo/viewport` requires an explicit bounded viewport and returns cluster-ready organization data.
- Map UI loads only the current viewport, aborts superseded requests, renders clusters and stable organization cards.
- GEO routes run through the existing API rate/concurrency/deadline guard.
- Ordinary WEB indexer remains isolated from `ORGANIZATION` events.

## Safety and integrity properties
- GEO reads canonical organizations only; staging/rejected rows are not index sources.
- Only `ACTIVE` organizations are returned by consumer GEO search.
- Organization Manticore schema has explicit upgrade handling for `city_key`.
- Rebuild is intentionally scoped to the organizations index and never drops/rebuilds the web index.
- Viewport queries cap returned candidates and cannot request an unbounded global result set.
- No address geocoding, FIAS/GAR import, paid ranking or organization claiming was added.

## Test coverage added
- GEO service input validation, candidate caps, Haversine distance and distance-aware ranking.
- Manticore GEO filter payload and backend result caps.
- Viewport bounds, bbox propagation and clustering.
- HTTP early rejection for invalid location, viewport and limit parameters.
- Organization index processor entity isolation, inactive deletion and stale-version ACK-without-write.
- Existing integration coverage asserts a WEB-only outbox consumer leaves `ORGANIZATION` events untouched.

## Executable gate limitation
An executable clone/build/test pass could not be run in the available container because DNS resolution for `github.com` failed (`Could not resolve host: github.com`). The sprint is therefore marked **PASS (code/static gate)** rather than claiming an unexecuted runtime PASS. Test code and integration fixtures are committed and ready to run in an environment with dependency/network access.

## Definition of Done
- [x] Dedicated ORGANIZATION consumer
- [x] Independent rebuild from canonical PostgreSQL
- [x] Stale-version protection
- [x] Text + city/category filters
- [x] Nearby bounded radius
- [x] Pre-backend coordinate/radius validation
- [x] Text/quality/source/distance ranking
- [x] Stable organization cards
- [x] Bounded viewport and cluster-ready output
- [x] Separate GEO route
- [x] WEB indexer isolation
- [x] Unit/integration test coverage committed
- [x] No Sprint 15 geocoding or paid ranking scope
- [x] No GitHub Actions/CI added
