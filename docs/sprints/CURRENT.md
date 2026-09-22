# CURRENT SPRINT

**Sprint:** 21 — Capacity Gate
**Status:** IN_PROGRESS

## Goal
Проверить фактическую производительность архитектуры на крупном корпусе и сформировать измеряемую модель роста до 10 млн документов. Решения о разделении сервисов, репликах или sharding допускаются только после benchmark и ADR; Sprint 21 не меняет архитектуру заранее.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–16 — PASS (code/static gate where noted in reports)
- Sprint 17 — PASS (code/static gate)
- Sprint 18 — PASS (code/static gate)
- Sprint 19 — PASS (code/static gate)
- Sprint 20 — PASS (code/static gate; report created)

## Allowed Work
- benchmark tooling for Search API and direct Manticore retrieval
- 1M-document benchmark preparation and corpus diagnostics
- target 10M capacity projection based on measured 1M/current-corpus values
- QPS, P50/P95/P99, error-rate and timeout measurement
- CPU/RAM/disk observation and database/index size reporting
- crawler/extractor/index throughput and queue/outbox backlog measurement
- Data Hub materialization throughput measurement
- GEO/address endpoint benchmark
- capacity snapshots stored as immutable benchmark records
- explicit thresholds and bottleneck classification
- ADR generation from measured benchmark results
- operator commands and runbook for repeatable benchmark execution
- tests for percentile math, projections, limits and decision rules

## Forbidden Work
- automatic architecture migration before benchmark evidence
- adding Kubernetes, Kafka, RabbitMQ or Redis
- adding Elasticsearch/OpenSearch
- speculative Manticore sharding or replicas without ADR
- benchmark writes against production canonical data without explicit isolated-mode confirmation
- fabricated benchmark results
- treating projected 10M numbers as measured values
- GitHub Actions/CI

## Definition of Done
- [ ] repeatable Search benchmark measures QPS, P50/P95/P99 and errors
- [ ] GEO/address benchmark uses the same bounded benchmark harness
- [ ] current corpus/document/index/database sizes are captured
- [ ] crawler/index throughput and crawl/outbox backlog are captured
- [ ] Data Hub materialization throughput is measurable
- [ ] 1M-document benchmark workflow is documented and isolated from production canonical state
- [ ] 10M model clearly distinguishes projection from measured results
- [ ] benchmark snapshots are immutable and comparable
- [ ] bottlenecks are classified by CPU/RAM/disk/search/database/queue signals
- [ ] capacity decision produces an ADR choice: stay single node / move crawler / shard search / add replica
- [ ] decision rules do not silently change architecture
- [ ] tests cover percentile calculations, error-rate, projections and decision thresholds
- [ ] no new infrastructure component is added before ADR
- [ ] no GitHub Actions/CI added
- [ ] Sprint 21 report created
