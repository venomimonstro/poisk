# CURRENT SPRINT

**Sprint:** 02 — Canonical Data + Queue
**Status:** IN_PROGRESS

## Goal
Создать каноническую модель данных и безопасные primitives для фоновой обработки: lease queue, idempotency и transactional outbox до появления crawler business logic.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS (GitHub Actions run `35456240270`)

## Allowed Work
- PostgreSQL schema
- domains / urls
- document_versions
- crawl_queue / crawl_history
- index_outbox
- audit_log
- system_settings
- queue/outbox repositories
- lease/idempotency/version primitives
- sqlc contract
- unit/integration tests для concurrency/data integrity

## Forbidden Work
- HTTP crawling/fetching
- robots/sitemap processing
- ranking/search API
- GEO business logic
- Answer Engine
- Redis/Kafka/RabbitMQ

## Definition of Done
- [ ] canonical tables созданы migrations
- [ ] constraints/indexes защищают invariants
- [ ] duplicate active crawl job не создаётся
- [ ] lease task выдаётся только одному worker
- [ ] expired lease возвращается в обработку
- [ ] retry/dead transition определены
- [ ] index_outbox имеет unique event invariant
- [ ] outbox lease безопасен для нескольких workers
- [ ] old entity version не может перезаписать new version на уровне application contract
- [ ] active queue отделена от crawl history
- [ ] migrations проходят на empty и Sprint 01 DB
- [ ] integration tests PASS
- [ ] Sprint 02 report создан
