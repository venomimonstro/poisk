# Sprint 03 Report — Scheduler + Crawler Security

**Status:** PASS

## Implemented
- deterministic URL normalization with invariant tests;
- DNS/IP SSRF target validator;
- rejection of loopback, private, link-local, multicast, documentation and metadata ranges;
- scheme/userinfo/port restrictions;
- PoiskBot robots.txt policy parser and sitemap discovery;
- streaming sitemap/sitemap-index parser with hard limits;
- bounded URL trap guards;
- global/domain scheduler primitives with crawl budgets;
- blocked-domain scheduling tests and scheduler priority tests.

## Security properties
- only HTTP/HTTPS targets are accepted;
- private/loopback/link-local/metadata targets are rejected before fetch;
- all DNS answers are checked, including mixed public/private answers;
- redirect targets are required to pass the same validator in Sprint 04 fetch path;
- crawler work remains bounded by domain policy, URL trap guards and sitemap limits.

## Verification
Repository contains unit/security tests for URL normalization, SSRF target validation, robots policy, sitemap limits, URL traps and scheduler budgets.

CI/GitHub Actions is intentionally not used for this project. Development continues directly in `main` as required by the repository workflow.

## Result
Sprint 03 Definition of Done is considered complete. Sprint 04 — HTTP Fetcher may start.
