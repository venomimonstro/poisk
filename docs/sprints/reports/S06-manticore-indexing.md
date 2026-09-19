# Sprint 06 — Manticore Indexing

**Status:** PASS (code/static gate)

## Implemented
- explicit versioned `web_documents` Manticore schema;
- bounded HTTP SQL client with request timeout, response hard limit and redirect refusal;
- correct parsing of Manticore `/sql?mode=raw` result sets;
- SQL-level remote error detection even when HTTP status is 200;
- deterministic REPLACE and idempotent DELETE paths;
- Manticore `entity_version` guard preventing older indexed versions from overwriting newer ones;
- PostgreSQL `document_content` persistence tied to `(url_id, version)`;
- extraction persistence and index-outbox creation in one PostgreSQL transaction;
- stale extraction suppression: historical extraction is retained but not enqueued when URL already advanced;
- entity-type-specific outbox lease for WEB_DOCUMENT workers;
- worker ordering: Manticore mutation first, `MarkProcessed` only after success;
- retry-safe failure handling with capped exponential delay;
- current-URL-version guard before applying an outbox UPSERT;
- current-version rebuild enumeration;
- noindex/thin current document removes stale index entry through idempotent DELETE.

## Race conditions closed
1. Extraction N finishes after URL is already N+1: N is stored but no index event is created.
2. Event N was created while current, then URL advances before lease: worker acknowledges N without mutating Manticore.
3. Manticore already contains version N+1 and receives N: client version guard skips mutation.

## Verification
Targeted package tests were added for SQL escaping, raw response parsing, stale version guard, DELETE, hard response limits, request timeout, worker acknowledgement ordering, retry behaviour and stale-event suppression. The execution environment still cannot perform a fresh dependency-backed repository-wide `go test ./...` without external dependency access; GitHub Actions/CI was intentionally not added per project requirement.

## Rollback
Sprint 06 is isolated to migration `000005_document_content.sql` and `internal/indexer/{manticore,source,worker,outbox}` additions/changes. Rollback requires reverting application commits before reverting the migration because indexed-source code depends on `document_content`.
