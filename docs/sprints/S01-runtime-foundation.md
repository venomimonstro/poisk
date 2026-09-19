# Sprint 01 — Runtime Foundation

## Goal
Поднять минимальный runtime проекта одной командой и обеспечить dependency-aware readiness.

## Depends On
Sprint 00 — PASS.

## Allowed Modules
- `cmd/app`
- `internal/platform/config`
- `internal/platform/health`
- `internal/platform/migrate`
- `web`
- `deploy`
- `db/migrations`

## Forbidden Work
- crawler business logic
- ranking
- GEO business logic
- Answer Engine
- неутверждённые infrastructure components

## DB Changes
Только bootstrap migration framework и PostGIS extension.

## Index Changes
Нет.

## API Changes
- `GET /health/live`
- `GET /health/ready`

## Tasks
- Docker Compose runtime
- Go modular runtime
- Next.js shell
- PostgreSQL/PostGIS
- Manticore
- Nginx
- typed config
- structured logs
- migration runner
- health endpoints

## Tests
- config validation
- liveness handler
- Go build/test/vet
- frontend TypeScript/build
- Docker Compose config validation

## Security Tests
- secrets не зашиты в image/repo
- backend/frontend с `no-new-privileges`
- внутренние БД не публикуются наружу

## Performance Tests
Не требуются до Sprint 10; только startup/readiness sanity.

## Data Migration
`000001_bootstrap.sql`.

## Rollback
Application image rollback; bootstrap migration не destructive.

## Definition of Done
См. `docs/sprints/CURRENT.md`.

## Deliverables
Runtime, health, migrations, frontend shell, Compose, quality gate.
