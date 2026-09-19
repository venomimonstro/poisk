# CURRENT SPRINT

**Sprint:** 01 — Runtime Foundation
**Status:** IN_PROGRESS

## Goal
Собрать минимальный production-minded runtime, который поднимается одной командой и проверяет зависимости через readiness.

## Depends On
Sprint 00 — PASS.

## Allowed Work
- Docker Compose
- Nginx
- Go application skeleton
- Next.js skeleton
- PostgreSQL/PostGIS
- Manticore
- migrations framework
- typed config
- structured logging
- `/health/live`
- `/health/ready`

## Forbidden Work
- crawler business logic
- ranking
- GEO business logic
- Answer Engine
- Redis/Kafka/Kubernetes/другие неутверждённые компоненты

## Definition of Done
- [ ] `docker compose up --build -d` запускает стек
- [ ] backend компилируется
- [ ] frontend компилируется
- [ ] PostgreSQL healthcheck работает
- [ ] Manticore healthcheck работает
- [ ] `/health/live` возвращает 200
- [ ] `/health/ready` проверяет критические зависимости
- [ ] config валидируется при старте
- [ ] structured logs работают
- [ ] unit tests PASS
- [ ] Sprint 01 report создан
