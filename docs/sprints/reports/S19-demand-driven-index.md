# Sprint 19 — Demand Driven Index Report

**Status:** PASS — code/static gate  
**Scope:** Query Gap, Search Coverage Score, demand scoring, bounded crawler feedback and manipulation protection.

## Implemented
- privacy-bounded query demand recording after successful Search responses;
- SHA-256 normalized query identity for raw signal buckets;
- 10-minute signal buckets with a hard cap of 3 observations per bucket;
- bucket aggregates freeze after the third observation, preventing repeated requests from inflating quality/result/spam sums;
- deterministic 0–100 Demand, Coverage, Quality, Freshness, Spam and Gap scores;
- Query Gap requires at least 3 independent time buckets plus weak coverage/quality before OPEN;
- representative query text is retained only after qualification and is bounded to 256 runes;
- Query Gap lifecycle supports WATCH / OPEN / RESOLVED / SUPPRESSED;
- operator `demandctl status|suppress|reopen` with immutable feedback audit;
- Search keeps coverage internals out of public JSON;
- real Freshness signal is calculated from indexed `fetched_at` age for TOP hits;
- result host observations resolve only against already-known ACTIVE/ALLOW-or-LIMITED canonical domains;
- unknown result hosts are not converted into domains or crawl jobs;
- separate `demand-worker` materializes feedback every 5 minutes;
- crawler feedback requires OPEN gap, minimum independent buckets and repeated domain observation across buckets;
- per-domain feedback boost is capped at +25 and expires after 6 hours;
- scheduler applies active feedback through a JOIN and does not overwrite the base `domains.demand_score`;
- feedback can only advance known domain recrawl to at most `now + 30 minutes`, never immediate arbitrary query-to-URL crawling;
- repeated worker runs in the same hourly idempotency window do not extend TTL or recrawl state;
- signal buckets/domain observations older than 7 days are pruned;
- `demand-worker` is a bounded independent Compose service;
- zero-result gaps without known domains remain diagnostics for seed discovery and never create URLs automatically.

## Security / correctness invariants
- no IP address, User-Agent or per-user query history is stored for demand scoring;
- one burst inside a single bucket cannot linearly increase demand;
- billing, referral and paid-plan state are not imported by or used in `internal/demand`;
- query text is never parsed as a crawl URL;
- crawler feedback targets only existing canonical `domain_id` values;
- BLOCK/REVIEW/PAUSED/DISABLED domains cannot receive active feedback;
- suppressing a gap immediately removes its active domain feedback;
- feedback actions have idempotency keys and immutable audit rows;
- feedback expiration automatically removes its influence from scheduler ordering.

## Tests / verification
Unit/integration test code covers:
- deterministic score boundaries and Query Gap qualification;
- same-bucket anti-spam cap;
- aggregate freeze after the third bucket observation;
- unknown host exclusion from domain observations;
- minimum independent buckets before feedback;
- feedback boost cap and recrawl bound;
- single-bucket spam cannot create feedback;
- active feedback affects scheduler order;
- expired feedback no longer affects scheduler order;
- real Freshness scoring from indexed `fetched_at`;
- unknown fetch time uses a neutral freshness value.

A local `go test ./...` execution was attempted, but the execution environment still cannot resolve `github.com`, so the repository/dependencies cannot be cloned into the runner. No GitHub Actions/CI was added. Sprint closure therefore uses the explicit code/static gate and does not claim an unexecuted runtime test pass.

## Migrations
- `000029_query_gaps.sql`
- `000030_query_gap_domain_observations.sql`
- `000031_query_gap_retention_index.sql`

## Rollback / recovery
Demand tables are auxiliary signals. They can be cleared or the `demand-worker` disabled without deleting canonical Search/Webmaster/GEO data. Scheduler falls back to base `domains.demand_score` when no active feedback exists. Search remains functional if Demand recording fails because recording is fail-open with a short independent deadline.

## Scope boundary
Sprint 20 Data Hub / city-category pages / programmatic SEO were not pulled into Sprint 19. Paid products, billing and referral data do not influence demand/gap/crawl scores. GitHub Actions/CI were not added.
