# poisk

Независимая поисковая система РФ: Web Search + Answer Engine + GEO + Webmaster.

Главный документ проекта: [`MASTER_PROJECT.md`](./MASTER_PROJECT.md).

## Правила разработки

1. Один активный спринт.
2. Перед изменениями читать `MASTER_PROJECT.md`, `docs/adr/*`, `docs/sprints/CURRENT.md`.
3. Стек и архитектуру не менять без ADR.
4. Следующий спринт начинается только после DoD текущего.
5. P0/P1 ошибки блокируют закрытие спринта.

## Локальный запуск

После Sprint 01:

```bash
cp .env.example .env
make dev
```

Проверка:

```bash
make health
make test
```
