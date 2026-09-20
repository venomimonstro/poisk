# Organizations Import

Sprint 13 imports organization datasets through staging. External files never write directly to canonical `organizations`.

## Input format

Operator files live under host `data/imports` and are mounted read-only as `/imports` in the backend container. The initial adapter is JSONL: one JSON object per line.

```json
{"source_record_id":"org-42","name":"Кофейня Ромашка","phone":"8 999 123-45-67","website":"romashka.example","address":"Москва, Тверская 1","category_key":"cafe","latitude":55.757,"longitude":37.615}
```

Required fields: `source_record_id`, `name`. Coordinates must either both be present or both be absent. Unknown JSON fields are rejected. Each source row is bounded to 64 KiB.

## Register a source

```bash
docker compose run --rm backend orgctl source catalog "Public catalog" 70
```

`source_key` is stable provenance. Do not reuse a key for an unrelated dataset.

## Dry run first

Place `batch-2026-09.jsonl` in `data/imports`, then:

```bash
docker compose run --rm backend orgctl import \
  catalog batch-2026-09 dry-run batch-2026-09.jsonl
```

The command stages every row, records malformed rows as bounded rejects, normalizes valid rows and creates a deterministic plan. DRY_RUN does not mutate canonical organizations.

Inspect a batch:

```bash
docker compose run --rm backend orgctl status <batch-id>
```

## Dedup rules

Automatic matching is intentionally conservative:

1. Existing `(source_key, source_record_id)` provenance wins.
2. Otherwise an exact normalized name plus exact phone is a strong match.
3. Otherwise exact normalized name plus exact website is a strong match.
4. Otherwise exact normalized name plus exact normalized address is a strong match.
5. No strong candidate means CREATE.
6. More than one strong candidate means REVIEW; the importer does not choose one automatically.

No fuzzy destructive merge is allowed in Sprint 13.

## Resolve ambiguous rows

For a REVIEW row choose one action:

```bash
docker compose run --rm backend orgctl review <staging-id> merge <place-id>
docker compose run --rm backend orgctl review <staging-id> create
docker compose run --rm backend orgctl review <staging-id> reject
```

The decision is audited. Manual reject moves the staging row to `REJECTED` with `MANUAL_REJECT`.

## Promote a dry run

A dry run can be promoted only after all REVIEW actions are resolved:

```bash
docker compose run --rm backend orgctl promote <batch-id>
```

The `organizations-worker` service picks up PLANNED/APPLY batches automatically.

## Direct APPLY

For trusted repeatable pipelines an operator may create an APPLY batch directly:

```bash
docker compose run --rm backend orgctl import \
  catalog batch-2026-10 apply batch-2026-10.jsonl
```

Ambiguous rows still block apply until manually resolved.

## Crash/resume semantics

Each applied plan row is one PostgreSQL transaction containing:

- canonical CREATE/UPDATE/NOOP;
- provenance link update;
- canonical `version` update when data changes;
- `ORGANIZATION` transactional outbox event;
- staging `APPLIED` state;
- plan `applied_at`;
- batch checkpoint and lease renewal;
- import audit event.

If the worker dies before COMMIT, none of those changes survive. If it dies after COMMIT, the checkpoint prevents the row from being applied again. An expired APPLY lease returns the batch to `PLANNED` for another worker.

The web-document indexer leases only `WEB_DOCUMENT` outbox events. `ORGANIZATION` events remain ready for the GEO/organizations indexer introduced in Sprint 14.

## Operational rules

- Keep input files immutable for the lifetime of an import batch.
- Use a new `external_batch_key` for a new snapshot/import attempt.
- Do not alter staging/canonical tables by hand to resolve duplicates; use the review command.
- Run DRY_RUN for a new source or source-format change before APPLY.
- Keep source IDs stable across snapshots; this is the strongest provenance signal.
- Review reject counts and OPEN reviews before promotion.
