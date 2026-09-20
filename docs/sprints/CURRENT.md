# CURRENT SPRINT

**Sprint:** 13 — Organizations Import
**Status:** IN_PROGRESS

## Goal
Построить безопасный и возобновляемый импорт организаций: внешние записи сначала попадают в staging, проходят source-specific parsing, validation и normalization, затем dedup/merge формируют каноническую OUR PLACE сущность. Ошибочная партия должна поддерживать dry-run, повторный запуск и продолжение после сбоя без загрязнения canonical данных.

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

## Allowed Work
- source adapter contracts for organization datasets
- import batch/job state and resumable checkpoints
- staging tables isolated from canonical organizations
- bounded raw payload storage / source row identity
- validation and rejection reasons
- normalization of names, phones, websites, categories and coordinates
- canonical OUR PLACE organization model
- source record → canonical place provenance
- deterministic exact/strong dedup rules
- review queue for ambiguous duplicates
- dry-run import and merge plans
- idempotent apply/resume/retry
- transactional canonical updates and index outbox events
- import metrics/audit events
- unit/integration/security tests for import isolation and idempotency

## Forbidden Work
- GEO search/ranking/nearby API
- organization cards in consumer search
- organization claiming
- FIAS/GAR address index or geocoding
- paid organization data APIs required for core import
- direct writes from external adapters to canonical organizations
- auto-merging ambiguous fuzzy matches without a deterministic confidence rule
- Redis/Kafka/RabbitMQ
- GitHub Actions/CI

## Definition of Done
- [ ] import batches are resumable and idempotent
- [ ] raw/source rows are isolated in staging before canonical mutation
- [ ] malformed rows are rejected with bounded diagnostics, not partially imported
- [ ] normalized organization records have deterministic source identity
- [ ] OUR PLACE canonical schema exists with versioning
- [ ] exact/strong duplicates merge deterministically
- [ ] ambiguous matches go to review instead of automatic destructive merge
- [ ] dry-run produces a stable merge/create/reject plan without canonical writes
- [ ] apply uses transactions and emits versioned index outbox events
- [ ] retry/resume cannot create duplicate canonical places
- [ ] provenance from source row to canonical place is queryable
- [ ] tests cover duplicate rows, worker crash/resume, dry-run and ambiguous match handling
- [ ] no GEO search/organization UI scope is pulled into Sprint 13
- [ ] no GitHub Actions/CI added
- [ ] Sprint 13 report created
