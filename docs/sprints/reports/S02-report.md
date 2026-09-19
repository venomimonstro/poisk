# Sprint 02 Report — Canonical Data + Queue

**Status:** PASS
**Validation run:** GitHub Actions `35456663094`

## Выполнено
- Канонические таблицы `domains`, `urls`, `document_versions`.
- Активная `crawl_queue` отдельно от `crawl_history`.
- `index_outbox` для PostgreSQL → Manticore synchronization.
- `audit_log`, `system_settings`.
- Partial unique index: одна активная crawl-задача на `url_id + generation`.
- Lease model для crawl queue и outbox.
- Ownership check: завершить lease может только worker-владелец.
- Expired lease recovery.
- Retry/DEAD transitions.
- Один outbox event на entity/version.
- Per-entity version ordering.
- Старые pending index events автоматически `SUPERSEDED`.
- Новая версия ждёт, пока уже leased старая версия завершится, что сохраняет ordering.
- sqlc contract (`sqlc.yaml`, typed query files).
- Go repositories для crawl queue и index outbox.
- CI-only PostgreSQL exposure только на `127.0.0.1`.

## Проверки
PASS:
- Go test/vet/build.
- sqlc v1.31.1 compile.
- migrations на чистой БД.
- runtime Compose.
- concurrent crawl queue leasing: 4 workers / 20 tasks без duplicate lease.
- idempotent enqueue.
- lease ownership.
- expired lease recovery.
- concurrent outbox leasing.
- outbox idempotency.
- entity-version conflict protection.
- stale event superseding.
- old/new outbox ordering.

## Ключевые инварианты
1. Нельзя создать две активные crawl-задачи одного поколения URL.
2. Worker не может завершить чужую lease.
3. Worker crash не теряет job.
4. Один entity/version имеет ровно одну index operation.
5. Старая версия не может быть обработана после более новой pending-версии.
6. PostgreSQL остаётся Source of Truth.

## P0/P1
Нет известных P0/P1 дефектов.

## Архитектурные отклонения
Нет.

## Следующий спринт
Sprint 03 — Scheduler + Crawler Security.
