# Sprint 12 — Maps

**Status:** PASS (code/static gate)

## Delivered
- PostgreSQL registry for immutable map versions, active/previous pointers and audit events.
- PMTiles manifest with version, SHA-256, sizes, bounds, zoom range, center, source key and attribution.
- Full integrity gate before register/activate/rollback.
- PMTiles v3 header validation for MVT archives, including magic/version, bounds, zoom and byte ranges.
- Strict style gate: MapLibre style v8, exactly one configured vector source, exact PMTiles binding, no remote dependencies/imports.
- Plain-text attribution validation to prevent HTML injection through map metadata.
- `mapctl manifest/register/activate/rollback/status` operator workflow.
- Offline OSM PBF → PMTiles workflow documented; no map generation occurs in the request path.
- Read-only `/api/map/config` endpoint exposing only the currently validated active version.
- Self-hosted `/maps/*` nginx serving with immutable cache policy and byte-range support; manifests are not publicly served.
- Backend and nginx receive the same read-only `data/maps` artifact directory.
- MapLibre GL JS + PMTiles frontend at `/map`, with server-controlled bounds/zoom/source and visible attribution.
- Search UI entry point to the map.
- Unit tests for manifest/file/header/style validation and service activation/rollback gates.
- Integration test for atomic active/previous version switching and rollback state.
- HTTP contract tests for map config availability/timeouts.
- No GEO/organizations scope, no paid map API dependency and no CI/GitHub Actions added.

## Safety / recovery
- Activated artifacts are immutable and mounted read-only.
- Activation validates the full SHA-256/file/header/style contract before changing the DB pointer.
- Runtime config re-checks presence, size, PMTiles header and style contract, failing closed if the active files disappear or are replaced incompatibly.
- Rollback validates the previous artifact set before switching.
- The previous version remains addressable by immutable filename; recovery does not require overwriting the failed release.
- Browser map style is rebound to the exact active same-origin PMTiles URL returned by the config API.

## Test gate limitation
The repository execution environment available during this sprint cannot resolve `github.com`, so a clean clone/dependency download and executable `go test`, integration database run and frontend build could not be performed here. The sprint therefore closes on a **code/static gate**, not a claimed runtime test pass. Unit/integration test code and deployment checks are committed and should be run in an environment with repository/dependency network access before public release.

## Operational verification before release
1. Apply migrations through `000009_map_versions.sql`.
2. Build/publish a real immutable PMTiles + style pair.
3. Generate the manifest with `mapctl manifest`.
4. Register and activate the version.
5. Verify `/api/map/config` returns the intended version.
6. Verify `/map` renders the intended viewport and attribution.
7. Send an HTTP `Range` request to the PMTiles URL and confirm `206 Partial Content`.
8. Activate a second test version and execute `mapctl rollback`; confirm active/previous pointers swap correctly.
9. Run `go test ./...`, integration tests with `TEST_DATABASE_URL`, and `npm run lint && npm run build`.

## Next sprint
Sprint 13 — Organizations Import: source adapters, staging, validation, normalization, OUR PLACE canonical model, deduplication, dry-run and resumable imports.
