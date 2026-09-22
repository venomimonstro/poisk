# Sprint 21 — Capacity Gate Report

**Status:** PASS — code/static implementation gate; measured production-like benchmark still required before commercial launch.  
**Commercial launch implication:** this report does **not** certify production capacity.

## Implemented

- repeatable bounded HTTP benchmark harness for Search, GEO and Address endpoints;
- QPS, request/error counts, error rate, P50/P95/P99 and max latency;
- database corpus snapshot: URLs, indexed documents, organizations, addresses and Data Hub published pages;
- crawl/outbox READY/LEASED/DEAD backlog snapshots;
- one-hour crawl/extraction/index/Data Hub throughput snapshots;
- PostgreSQL database byte measurement;
- measured Manticore storage input plus explicit linear 10M storage projection labelled `PROJECTED_LINEAR_STORAGE_ONLY`;
- client benchmark resource diagnostics separated from server resource evidence;
- **mandatory measured server CPU/RAM/disk evidence** before a completed capacity snapshot is accepted;
- direct Manticore web-document count probe;
- PostgreSQL indexed-document vs Manticore document-count mismatch classification with tolerance;
- immutable `capacity_benchmark_runs`, `capacity_snapshots` and ADR decisions;
- DB triggers reject mutation/deletion of completed snapshots and ADR decisions;
- `LIVE_READONLY` and explicit `ISOLATED_1M` benchmark modes;
- `ISOLATED_1M` refuses to run unless the indexed corpus is at least 1,000,000 documents;
- benchmark inputs are bounded files; endpoint paths must remain relative and within allowed API prefixes;
- deterministic bottleneck classification for Search SLO, CPU/RAM/disk, crawl/index backlog, QPS capacity, storage projection and index-count mismatch;
- evidence-based ADR choices: `STAY_SINGLE_NODE`, `MOVE_CRAWLER`, `ADD_REPLICA`, `SHARD_SEARCH`;
- no automatic architecture migration is performed by benchmark tooling;
- `capacityctl benchmark/status/adr` runtime is wired;
- no GitHub Actions/CI added.

## Critical audit repairs made during Sprint 21

1. `capacity_manticore.go` referenced a missing `manticore.Client.CountDocuments()` method. This was a compile blocker in the Capacity path. A bounded implementation and unit test were added.
2. `INDEX_COUNT_MISMATCH` existed in classification logic but the benchmark did not populate `DBIndexedDocuments` or `ManticoreDocuments`. The benchmark now measures both and persists the delta.
3. completed benchmark snapshots previously allowed server CPU/RAM/disk to be absent. `CAPACITY_SERVER_METRICS_FILE` is now mandatory for a completed snapshot.
4. legacy Manticore `web_documents` indexes could lack the later `fetched_at` attribute used by freshness scoring. `EnsureSchema()` now repairs both missing `authority_score` and missing `fetched_at` attributes before indexing.

## Tests / static verification

Code contains tests for:
- percentile/error-rate math;
- storage projection labelling and validation;
- bottleneck classification;
- index-count mismatch tolerance;
- deterministic architecture recommendation thresholds;
- Manticore document-count parsing;
- Manticore legacy schema upgrade of required web attributes;
- repository snapshot/ADR invariants through integration coverage.

The current execution environment has historically been unable to resolve external Go dependencies from `github.com`, so this report does not claim an unexecuted full `go test ./...`, migration run or 1M runtime benchmark.

## Required external launch evidence

Before commercial launch, run on a production-like host:

1. clean backend/frontend build;
2. empty-database migration and previous-version upgrade migration;
3. full unit/integration/security tests;
4. `capacityctl benchmark` with measured server resource file;
5. isolated >=1M indexed-document benchmark;
6. review PostgreSQL/Manticore document-count consistency;
7. persist the immutable capacity snapshot;
8. create the explicit capacity ADR;
9. repeat restore/rebuild drill after the capacity run.

These are launch gates. They must not be replaced by projected or fabricated results.

## Architecture decision

No sharding, replicas, Kafka/Redis/Kubernetes or service split was added in Sprint 21. The architecture remains a modular monolith until measured evidence and an ADR justify a change.
