# Sprint 13 — Organizations Import

**Status:** PASS (code/static gate)

## Delivered
- Canonical OUR PLACE organization schema with explicit `place_id`, entity `version`, status, normalized identity fields, coordinates, source count and provenance.
- Source registry with stable `source_key` and trust metadata.
- Isolated import batch and staging tables; external rows never write directly to canonical organizations.
- Bounded JSONL adapter: one row at a time, strict JSON contract, 64 KiB row limit, malformed rows become bounded REJECTED records instead of aborting the entire batch.
- Deterministic normalization for names, Russian-style phone input, IDN websites, addresses, categories and coordinates.
- Stable normalized payload SHA-256 for idempotency while preserving the original source payload in staging.
- Idempotent batch identity `(source_key, external_batch_key)` and idempotent staged source-row identity.
- Deterministic planner with source identity precedence and only strong exact automatic matches:
  - source identity;
  - normalized name + phone;
  - normalized name + website;
  - normalized name + normalized address.
- Multiple strong matches are never auto-merged; they become REVIEW records with candidate provenance.
- Stable CREATE / UPDATE / NOOP / REJECT / REVIEW plan records and plan hashes.
- Manual review decisions: merge, create new or reject, all audited.
- DRY_RUN workflow with zero canonical writes and explicit promotion to APPLY only after unresolved reviews are cleared.
- Crash-safe APPLY worker with PostgreSQL leases, expired-lease recovery and per-source advisory locks.
- One-plan-per-transaction apply semantics: canonical mutation, provenance, entity version, ORGANIZATION outbox event, staging state, plan state, checkpoint and audit event commit together.
- Resume semantics prevent already committed rows from being applied twice after worker restart.
- Source identity conflicts fail closed instead of silently overwriting a newer source snapshot.
- Organization provenance is queryable through `organization_source_links`.
- `orgctl` operator commands for source registration, import, plan/status, review and dry-run promotion.
- Import files are available only through the read-only `/imports` mount; no public organization-import HTTP endpoint exists.
- Dedicated `organizations-worker` runtime and Compose service.
- Web-document indexer now leases only `WEB_DOCUMENT` outbox events; `ORGANIZATION` events remain READY for Sprint 14 consumer.
- Fixed a pre-existing outbox conflict-target regression introduced after migration `000004_outbox_event_identity.sql`, including Webmaster DELETE events.
- Unit test code for normalization, bounded staging and deterministic planner decisions.
- Integration test code for dry-run isolation, repeated source NOOP, ambiguous-review gate, crash/lease-expiry resume and organization outbox creation.
- Integration test code proving entity-scoped web indexer leases do not consume organization events.
- No GEO search, organization claiming, FIAS/geocoding, paid data dependency or CI/GitHub Actions added.

## Safety / recovery
- Source payloads remain isolated in staging before planning.
- Invalid rows carry bounded diagnostic codes/details and cannot partially mutate canonical data.
- Automatic merge uses only deterministic strong rules; ambiguity is surfaced for human review.
- APPLY is lease-owned and checkpointed at committed plan boundaries.
- A worker crash before COMMIT leaves no partial canonical mutation; a crash after COMMIT resumes after the committed checkpoint.
- Source identity is serialized with a PostgreSQL advisory transaction lock during apply.
- Canonical changes create versioned transactional outbox events, but Sprint 13 deliberately does not consume ORGANIZATION events into the search index.

## Test gate limitation
The available execution environment still cannot resolve `github.com`, so a clean dependency download and executable `go test`, integration PostgreSQL run and container build could not be performed here. Sprint 13 therefore closes on a **code/static gate**, not a claimed runtime test pass. The committed unit/integration tests must be run in an environment with repository/dependency access before a public release.

## Runtime verification before release
1. Apply migrations through `000010_organizations_import.sql`.
2. Run `go test ./...`.
3. Run integration tests with `TEST_DATABASE_URL`.
4. Register a temporary source and import a mixed valid/invalid JSONL file in DRY_RUN.
5. Confirm canonical `organizations` remains unchanged during dry-run.
6. Create an ambiguous duplicate and confirm APPLY cannot lease the batch before review resolution.
7. Promote/apply the batch, terminate the worker after one committed row, expire/requeue the lease and confirm a second worker resumes without duplicate places.
8. Confirm `ORGANIZATION` outbox events remain READY while the current web indexer continues processing only `WEB_DOCUMENT` events.

## Next sprint
Sprint 14 — GEO Search: organizations index, PostGIS/geospatial retrieval, city/category/nearby search, cards, clustering and GEO Query Planner.
