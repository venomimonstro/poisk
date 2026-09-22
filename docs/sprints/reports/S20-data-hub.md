# Sprint 20 — Data Hub Report

**Status:** PASS — code/static gate  
**Scope:** canonical Data Hub, bounded programmatic SEO, privacy-safe trends and publication controls.

## Implemented
- stable `datahub_pages` identity with deterministic slugs and canonical paths;
- versioned immutable page snapshots and monotonic rollback-by-cloning;
- publication lifecycle `DRAFT / PUBLISHED / SUPPRESSED`;
- DB-enforced manual suppression that a builder cannot accidentally override;
- city, category and city+category pages built only from ACTIVE canonical organizations;
- organization pages with canonical identity/source evidence thresholds;
- website pages requiring canonical organization↔domain provenance and at least one indexed URL;
- thin-page gates for directory and entity pages;
- deterministic related-page links with bounded result count;
- privacy-safe daily trends sourced only from qualified aggregated Query Gaps;
- no per-user query history, IP or User-Agent stored for Data Hub trends;
- read-only Data Hub API with bounded filters/cursor pagination;
- canonical/meta/robots output for published pages only;
- server-rendered `/data/...` pages and `/data` trend landing;
- XML sitemap index and bounded 500-URL sitemap shards;
- `robots.txt` sitemap declaration;
- persistent build cursors for directories, organizations and websites;
- resumable Data Hub worker with resource-pressure gate;
- `datahubctl status/build/suppress/reopen/rollback` operator controls;
- homepage navigation to Data Hub;
- no billing/paid-plan dependency in organic directory publication or ordering.

## Safety / quality invariants
- generated pages cannot publish below the page-type evidence threshold;
- lost evidence moves previously published pages to `SUPPRESSED`;
- manually suppressed pages stay suppressed across rebuilds;
- external website links are rendered only for validated HTTP/HTTPS URLs;
- dynamic Data Hub routes do not render arbitrary HTML from canonical data;
- structured-data JSON is escaped before embedding;
- only `PUBLISHED` pages enter public page lookup and sitemap output;
- trend retention is aggregate-only and bounded;
- auxiliary AI summaries were intentionally not shipped in Sprint 20. This keeps the stronger invariant that no generated prose can add facts outside canonical aggregates.

## Tests / verification
Added coverage for:
- deterministic/bounded slug rules;
- stable canonical identities;
- thin city/category combination suppression;
- stricter city evidence threshold;
- organization identity/source evidence gate;
- website linked-organization gate;
- pagination parameter bounds;
- resumable organization materialization across saved cursor checkpoints.

A full local build/test execution is still unavailable in the current tool environment because direct DNS access to GitHub/dependency sources is blocked. No GitHub Actions/CI was added. Closure therefore uses the explicit code/static gate model used by earlier environment-constrained sprints and does not claim an unexecuted runtime test pass.

## Migrations
- `000032_data_hub.sql`
- `000033_data_hub_manual_suppression.sql`
- `000034_data_hub_build_state.sql`

## Recovery / rollback
- canonical Search/GEO/Webmaster data is never owned by Data Hub;
- Data Hub can be rebuilt from canonical sources;
- page versions retain historical snapshots;
- rollback creates a new monotonic version from a chosen historical snapshot instead of rewinding version numbers;
- operator suppression is reversible and audit-visible;
- failure during a batch leaves the saved cursor at the last completed item, so restart resumes without starting the whole corpus again.

## Scope boundary
Sprint 21 benchmark/capacity/sharding decisions were not pulled into Sprint 20. No new infrastructure component and no GitHub Actions/CI were added.
