# Capacity Gate Runbook

Sprint 21 measures the existing architecture. It does **not** authorize an infrastructure change by itself.

## Safety rules

1. `LIVE_READONLY` benchmarks only existing HTTP read endpoints and reads PostgreSQL diagnostics. It must not seed or mutate canonical Search/GEO data.
2. A 1M corpus benchmark must run in a **separate Docker Compose project with separate PostgreSQL and Manticore volumes**.
3. Never restore or generate a synthetic 1M corpus into the production project.
4. `CAPACITY_MANTICORE_BYTES` must be measured from the Manticore data volume. Do not enter a guessed value.
5. `projection_10m.kind=PROJECTED_LINEAR_STORAGE_ONLY` is a projection, not a measured 10M result.
6. Architecture changes require a separately recorded human ADR decision.
7. The one-off benchmark container's cgroup is **not** treated as backend/Manticore CPU/RAM. Server resource bottlenecks require a separate measurement for the same benchmark window.

## Workload files

Reference workloads are versioned in:

- `deploy/capacity/search.txt`
- `deploy/capacity/geo.txt`
- `deploy/capacity/address.txt`
- `deploy/capacity/server-metrics.example.json`

They are copied into the backend runtime image under `/app/deploy/capacity/`. Replace/add cases only with valid bounded production routes. Do not add authentication secrets or user-specific data to these files.

## Measure Manticore bytes

Inside the target Compose project:

```sh
docker compose exec -T manticore sh -c "du -sb /var/lib/manticore | awk '{print \$1}'"
```

Record that integer as `CAPACITY_MANTICORE_BYTES` for the benchmark run.

## Server resource measurement

CPU/RAM of the short-lived `capacityctl` container are recorded only as benchmark-generator diagnostics. They are not used to classify server pressure.

For CPU/RAM/disk bottleneck classification, create a JSON file from the monitoring system or host-level sampling covering the same benchmark window:

```json
{
  "cpu_percent": 68.4,
  "ram_percent": 73.2,
  "disk_percent": 61.0,
  "disk_available_bytes": 412316860416,
  "source": "host monitoring average/max for backend+postgres+manticore during benchmark window"
}
```

Percentages are normalized to 0–100 for the allocated search host/capacity under test. Mount this file read-only and set `CAPACITY_SERVER_METRICS_FILE`. If it is omitted, the snapshot explicitly states that server CPU/RAM/disk values were not provided, and those signals are not used for bottleneck classification.

## Current-corpus read-only benchmark

The backend image is distroless, so use `docker compose run` instead of trying to open a shell in it.

```sh
docker compose run --rm \
  -v "$PWD/server-metrics.json:/capacity/server-metrics.json:ro" \
  -e CAPACITY_MODE=LIVE_READONLY \
  -e CAPACITY_BASE_URL=http://backend:8080 \
  -e CAPACITY_DURATION_SECONDS=60 \
  -e CAPACITY_CONCURRENCY=16 \
  -e CAPACITY_TARGET_QPS=100 \
  -e CAPACITY_MANTICORE_BYTES=<measured integer> \
  -e CAPACITY_SEARCH_QUERIES=/app/deploy/capacity/search.txt \
  -e CAPACITY_GEO_URLS=/app/deploy/capacity/geo.txt \
  -e CAPACITY_ADDRESS_URLS=/app/deploy/capacity/address.txt \
  -e CAPACITY_SERVER_METRICS_FILE=/capacity/server-metrics.json \
  backend capacityctl benchmark current-baseline
```

The immutable snapshot records:

- Search/GEO/Address request count, error rate, QPS and P50/P95/P99;
- indexed document count and canonical corpus counts;
- PostgreSQL size and measured Manticore size;
- crawl/outbox backlog;
- crawl/extraction/index/Data Hub last-hour throughput;
- explicit server resource measurements when provided;
- benchmark-client cgroup diagnostics separately;
- explicit linear storage projection to 10M documents;
- classified bottlenecks.

## Isolated 1M workflow

Create an isolated environment. The exact project name is not important; separation is.

```sh
docker compose -p poisk-capacity-1m --env-file .env.capacity up -d
```

Requirements for `.env.capacity`:

- a separate PostgreSQL database/volume;
- a separate Manticore volume;
- no production database DSN;
- no production writable filesystem mount;
- normal crawler/import/indexer safety limits remain enabled.

Apply migrations in that isolated project. Populate the corpus using the same canonical ingestion/indexing pipeline used by Poisk (restored sanitized benchmark corpus, controlled crawler corpus, or approved import pipeline). Sprint 21 deliberately does not include a command that can generate one million canonical rows against an arbitrary database.

Before benchmarking, verify the isolated database reports at least **1,000,000** rows where `urls.index_status='INDEXED'`. `capacityctl` also checks this and refuses `ISOLATED_1M` below that threshold.

Measure isolated Manticore bytes and create server metrics for the isolated benchmark window, then run:

```sh
docker compose -p poisk-capacity-1m --env-file .env.capacity run --rm \
  -v "$PWD/server-metrics.json:/capacity/server-metrics.json:ro" \
  -e CAPACITY_MODE=ISOLATED_1M \
  -e CAPACITY_BASE_URL=http://backend:8080 \
  -e CAPACITY_DURATION_SECONDS=180 \
  -e CAPACITY_CONCURRENCY=32 \
  -e CAPACITY_TARGET_QPS=100 \
  -e CAPACITY_MANTICORE_BYTES=<measured isolated Manticore bytes> \
  -e CAPACITY_SEARCH_QUERIES=/app/deploy/capacity/search.txt \
  -e CAPACITY_GEO_URLS=/app/deploy/capacity/geo.txt \
  -e CAPACITY_ADDRESS_URLS=/app/deploy/capacity/address.txt \
  -e CAPACITY_SERVER_METRICS_FILE=/capacity/server-metrics.json \
  backend capacityctl benchmark isolated-1m
```

If the isolated corpus has fewer than one million indexed documents, the command fails instead of labeling the run `ISOLATED_1M`.

## Compare immutable snapshots

```sh
docker compose run --rm backend capacityctl status 20
```

Compare measured values, not only projections. In particular compare Search P95/P99, error rate, QPS, CPU/RAM, disk, crawl/extract/index throughput and queue backlogs.

## Architecture decision

Allowed ADR choices are exactly:

- `STAY_SINGLE_NODE`
- `MOVE_CRAWLER`
- `SHARD_SEARCH`
- `ADD_REPLICA`

Example:

```sh
docker compose run --rm backend capacityctl adr 12 STAY_SINGLE_NODE operator "Measured 1M workload stays inside latency, error, memory and disk budgets; no additional infrastructure is justified yet."
```

This stores one immutable ADR decision for the snapshot and prints Markdown describing the measured evidence. The benchmark classifier never deploys or changes architecture automatically.

## Interpreting the 10M model

The 10M projection linearly scales **only measured PostgreSQL + Manticore storage bytes per indexed document**. It does not claim linear QPS, latency, CPU, RAM, crawler throughput or database contention. Those require a real larger-corpus benchmark before an architecture migration.

## Data Hub throughput

`capacityctl benchmark` records `datahub_publication_events` over the last hour as `datahub_changes_per_second`. For a meaningful materialization measurement in the isolated project, run the Data Hub worker during a controlled rebuild and compare snapshots before/after. Do not trigger a mass Data Hub rebuild on production solely for benchmarking.

## Failure handling

A failed benchmark run is marked `FAILED`; no immutable capacity snapshot is created. A completed snapshot cannot be updated or deleted. Re-run with a new label after correcting the issue so comparison history remains intact.
