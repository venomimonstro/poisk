# Sprint 11 Report — Webmaster Free

**Status:** PASS (code/static gate; executable test/migration gate unavailable in current tool environment)

## Implemented
- local Webmaster registration/login/logout;
- PBKDF2-HMAC-SHA256 password hashes with random salts;
- opaque random session tokens with only SHA-256 token hashes persisted;
- database-enforced maximum of 10 active sessions per user;
- canonical public site-origin validation through the crawler SSRF policy;
- tenant-safe site ownership checks in repository SQL;
- ownership verification by DNS TXT, HTML file, or meta tag;
- replay-safe expiring verification challenges and audit events;
- verified-site sitemap submission with crash-safe lease/retry worker;
- bounded nested sitemap parsing through existing crawler sitemap limits;
- same-host gates for sitemap contents and redirect targets;
- canonical URL SUBMIT/REINDEX into crawl queue;
- versioned URL DELETE through transactional index outbox, never direct Manticore mutation;
- global domain policy enforcement for Webmaster URL and sitemap work;
- URL crawl/index diagnostics;
- daily impressions, indexed-result clicks, CTR and Answer citation aggregates;
- public click tracking without open redirects;
- Webmaster API protected by the existing rate/deadline/concurrency guard;
- Compose runtime for `webmaster-worker`.

## Security / regression coverage added
- weak-password rejection and password round-trip;
- opaque token hashing;
- root-origin/SSRF validation;
- exact DNS/file/meta ownership proof checks;
- cross-host ownership redirect rejection;
- cross-tenant site isolation;
- cross-host URL submission rejection;
- trailing/unknown JSON rejection;
- sitemap URLSet/SitemapIndex processing and cross-host rejection;
- integration tests for case-insensitive email uniqueness;
- integration test for database session cap;
- integration tests for canonical crawl queue and versioned delete outbox;
- integration tests for Webmaster metrics;
- integration test for sitemap lease and blocked-domain policy.

## Important hardening fixes found during the sprint
- replaced invalid PostgreSQL `UNIQUE(lower(email))` table constraint with an expression unique index;
- prevented sitemap resubmission from stealing an active worker lease;
- prevented existing canonical URLs from crossing verified `domain_id` boundaries;
- blocked Webmaster work when canonical domain policy is `BLOCK` or domain is not active;
- restricted click metrics to canonical URLs with `index_status='INDEXED'`;
- fixed multiple test-fixture escaping errors before treating tests as valid.

## Validation limitation
A direct local clone/build/test attempt failed because the execution environment cannot resolve `github.com` (`Could not resolve host: github.com`). Per project constraints, no GitHub Actions/CI workflow was added. Therefore this report records a code/static gate and does **not** claim that `go test ./...`, integration tests, or migrations were executed in this environment.

## Recovery / rollback
- Webmaster mutations are isolated in migrations `000007_webmaster_free.sql` and `000008_webmaster_session_cap.sql` plus additive runtime code.
- URL deletion remains recoverable through canonical versioning/outbox semantics.
- Sitemap jobs are lease/retry-safe and do not directly mutate the search index.
- Search remains usable if Webmaster worker is stopped; it only delays sitemap ingestion.
