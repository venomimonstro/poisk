# Capacity Gate Runbook

Sprint 21 measures the existing architecture. It does **not** authorize an infrastructure change by itself.

## Safety rules

1. `LIVE_READONLY` benchmarks only existing HTTP read endpoints and reads PostgreSQL diagnostics. It must not seed or mutate canonical Search/GEO data.
2. A 1M corpus benchmark must run in a **separate Docker Compose project with separate PostgreSQL and Manticore volumes**.
3. Never restore or generate a synthetic 1M corpus into the production project.
4. `CAPACITY_MANTICORE_BYTES` must be measured from the Manticore data volume. Do not enter a guessed value.
5. `projection_10m.kind=PROJECTED_LINEAR_STORAGE_ONLY` is a projection, not a measured 10M result.
6. Architecture changes require a separately recorded human ADR decision.

## Workload files

Reference workloads are versioned in:

- `deploy/capacity/search.txt`
- `deploy/capacity/geo.txt`
- `deploy/capacity/address.txt`

Replace/add cases only with valid bounded production routes. Do not add authentication secrets or user-specific data to these files.

## Measure Manticore bytes

Inside the target Compose project:

```sh
docker compose exec -T manticore sh -c "du -sb /var/lib/manticore | awk '{print \$1}'"
```

Record that integer as `CAPACITY_MANTICORE_BYTES` for the benchmark run.

## Current-corpus read-only benchmark

Example environment:

```sh
export CAPACITY_MODE=LIVE_READONLY
export CAPACITY_BASE_URL=http://backend:8080
export CAPACITY_DURATION_SECONDS=60
export CAPACITY_CONCURRENCY=16
export CAPACITY_TARGET_QPS=100
export CAPACITY_MANTICORE_BYTES=<measured integer>
export CAPACITY_SEARCH_QUERIES=/app/deploy/capacity/search.txt
export CAPACITY_GEO_URLS=/app/deploy/capacity/geo.txt
export CAPACITY_ADDRESS_URLS=/app/deploy/capacity/address.txt
export CAPACITY_DISK_PATH=/
```

Run inside the backend image/container where the workload files are available:

```sh
./app capacityctl benchmark current-baseline
```

The immutable snapshot records:

- Search/GEO/Address request count, error rate, QPS and P50/P95/P99;
- indexed document count and canonical corpus counts;
- PostgreSQL size and measured Manticore size;
- crawl/outbox backlog;
- crawl/index/Data Hub last-hour throughput;
- cgroup CPU/RAM and filesystem disk pressure when available;
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

Then run with:

```sh
export CAPACITY_MODE=ISOLATED_1M
export CAPACITY_BASE_URL=http://backend:8080
export CAPACITY_DURATION_SECONDS=180
export CAPACITY_CONCURRENCY=32
export CAPACITY_TARGET_QPS=100
export CAPACITY_MANTICORE_BYTES=<measured isolated Manticore bytes>
export CAPACITY_SEARCH_QUERIES=/app/deploy/capacity/search.txt
export CAPACITY_GEO_URLS=/app/deploy/capacity/geo.txt
export CAPACITY_ADDRESS_URLS=/app/deploy/capacity/address.txt
./app capacityctl benchmark isolated-1m
```

If the isolated corpus has fewer than one million indexed documents, the command fails instead of labeling the run `ISOLATED_1M`.

## Compare immutable snapshots

```sh
./app capacityctl status 20
```

Compare measured values, not only projections. In particular compare Search P95/P99, error rate, QPS, CPU/RAM, disk, outbox backlog and throughput.

## Architecture decision

Allowed ADR choices are exactly:

- `STAY_SINGLE_NODE`
- `MOVE_CRAWLER`
- `SHARD_SEARCH`
- `ADD_REPLICA`

Example:

```sh
./app capacityctl adr 12 STAY_SINGLE_NODE operator "Measured 1M workload stays inside latency, error, memory and disk budgets; no additional infrastructure is justified yet."
```

This stores one immutable ADR decision for the snapshot and prints Markdown describing the measured evidence. The benchmark classifier never deploys or changes architecture automatically.

## Interpreting the 10M model

The 10M projection linearly scales **only measured PostgreSQL + Manticore storage bytes per indexed document**. It does not claim linear QPS, latency, CPU, RAM, crawler throughput or database contention. Those require a real larger-corpus benchmark before an architecture migration.

## Data Hub throughput

`capacityctl benchmark` records `datahub_publication_events` over the last hour as `datahub_changes_per_second`. For a meaningful materialization measurement in the isolated project, run the Data Hub worker during a controlled rebuild and compare snapshots before/after. Do not trigger a mass Data Hub rebuild on production solely for benchmarking.

## Failure handling

A failed benchmark run is marked `FAILED`; no immutable capacity snapshot is created. A completed snapshot cannot be updated or deleted. Re-run with a new label after correcting the issue so comparison history remains intact.
