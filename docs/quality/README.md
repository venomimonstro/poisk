# Search Quality Lab

Этот каталог хранит воспроизводимую оценку качества поиска.

## Golden set

`golden.seed.json` использует schema version 1. Каждый запрос имеет стабильный `id`, текст запроса, теги и список judgments по URL.

Шкала relevance:

- `0` — нерелевантно;
- `1` — частично полезно;
- `2` — релевантно и хорошо отвечает запросу;
- `3` — эталонный / наиболее ожидаемый результат.

Неоднозначные информационные и коммерческие запросы нельзя размечать выдуманными URL. Их judgments добавляются только после появления достаточно репрезентативного индексного корпуса и ручной проверки выдачи.

## Запуск

```bash
go run ./cmd/quality
```

Переменные окружения:

```text
MANTICORE_HOST=localhost
MANTICORE_HTTP_PORT=9308
QUALITY_GOLDEN_PATH=docs/quality/golden.seed.json
QUALITY_THRESHOLDS_PATH=docs/quality/thresholds.json
```

Команда печатает JSON с per-query metrics, aggregate summary и gate result. Ненулевой exit code означает, что quality gate не пройден.

## Fail-safe правила

- запросы без judgments не участвуют в NDCG/MRR/Recall и не создают ложное качество;
- если нет ни одного judged query, quality gate всегда FAIL;
- неизвестные поля и неверные grades в golden JSON отклоняются;
- thresholds обязаны быть в диапазоне `0..1`;
- изменение thresholds должно быть явным commit, а не автоматической подстройкой под текущий результат;
- ухудшение метрики нельзя скрывать расширением допустимого порога без объяснения причины.

## Масштабирование набора

Цель Search Quality Lab: 500 → 1000 → 5000 реально размеченных запросов. Добавлять запросы нужно по сегментам: informational, navigational, commercial, long-tail, typo/layout/translit, freshness, comparison и позднее GEO. При расширении набора сохраняются старые `id`, чтобы отчёты были сопоставимы во времени.
