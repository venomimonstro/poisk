# Sprint 10 Report — Performance / Load Protection

**Status:** PASS (code/static gate; no CI added)

## Implemented
- explicit request deadline for Search and Answer API;
- bounded API request concurrency with deterministic `503 overloaded`;
- bounded per-client token bucket with `429 rate_limited` and `Retry-After`;
- bounded rate-limiter client cardinality with idle eviction and oldest-entry eviction;
- query complexity guard before search backend;
- identical concurrent backend searches coalesced through local single-flight;
- separate global backend-search concurrency semaphore;
- bounded search cache cardinality and expiry regression tests;
- bounded latency ring recorder with P50/P95/P99 snapshots;
- `/health/perf` latency diagnostics;
- explicit HTTP mapping for query complexity and request deadline errors;
- load-protection settings moved to typed configuration and `.env.example`.

## Regression coverage added
- token bucket burst/refill;
- limiter max-client cardinality and idle eviction;
- 429/503/deadline middleware behavior;
- single-flight coalescing;
- backend concurrency ceiling and context cancellation;
- pathological token/operator/depth/control-character queries;
- cache cardinality/expiry;
- Search and Answer timeout/complexity HTTP contracts;
- latency ring capacity and percentile calculation.

## Operational defaults
- `API_REQUEST_DEADLINE=3s`
- `API_MAX_CONCURRENT=64`
- `SEARCH_BACKEND_MAX_CONCURRENT=32`
- `API_RATE_PER_SECOND=10`
- `API_RATE_BURST=20`
- `API_RATE_MAX_CLIENTS=10000`
- `API_RATE_IDLE_TTL=10m`

## Notes
- No Redis/Kafka/RabbitMQ/Kubernetes/microservice dependency added.
- No GitHub Actions or CI workflow added.
- The repository connector does not provide an executable Go build environment, so this report records a code/static gate rather than claiming a locally executed `go test ./...` run.
