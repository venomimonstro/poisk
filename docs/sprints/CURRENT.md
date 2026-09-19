# CURRENT SPRINT

**Sprint:** 06 — Manticore Indexing
**Status:** IN_PROGRESS

## Goal
Реализовать идемпотентную индексацию WEB_DOCUMENT из PostgreSQL outbox в Manticore Search: schema, version ordering, UPSERT/DELETE, retry-safe worker primitives и rebuild path без ranking/search API.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS
- Sprint 04 — PASS
- Sprint 05 — PASS (code/static gate; no CI added)

## Allowed Work
- Manticore web index schema
- HTTP SQL transport with bounded timeouts/body
- typed web-document payload
- UPSERT/REPLACE and DELETE operations
- entity-version ordering/idempotency guards
- PostgreSQL document loader for latest READY extraction
- index_outbox worker primitives
- rebuild/bulk replay command primitives
- transport/retry/idempotency tests

## Forbidden Work
- ranking experiments/BM25 tuning
- Search API / SERP
- query understanding
- Answer Engine
- GEO business logic
- JavaScript rendering
- introducing Redis/Kafka/vector DB/microservices

## Definition of Done
- [ ] WEB_DOCUMENT schema is explicit and versioned
- [ ] Manticore client has bounded request timeout and response limit
- [ ] UPSERT is deterministic for same entity/version
- [ ] stale entity versions cannot overwrite newer indexed versions
- [ ] DELETE is idempotent
- [ ] latest extracted document can be loaded from PostgreSQL
- [ ] outbox event can be applied and marked processed only after index success
- [ ] transient index failure is retry-safe
- [ ] rebuild path can enumerate canonical indexable documents
- [ ] tests cover SQL escaping, version ordering, delete and transport errors
- [ ] no GitHub Actions/CI added
- [ ] Sprint 06 report created
