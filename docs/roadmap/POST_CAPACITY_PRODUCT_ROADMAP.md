# Post-Capacity Product Roadmap

This roadmap extends the product after Sprint 21. Only one sprint may be active at a time. Sprint 21 remains the active sprint until its measured gate is complete.

## Sprint 22 — Unified Identity + Webmaster Cabinet

### Goal
Create one canonical consumer account/session layer for public products while keeping Admin isolated, then deliver a complete Webmaster self-service cabinet.

### Scope
- canonical consumer users and HttpOnly/Secure/SameSite session cookies;
- email verification, password reset, session list/revocation and CSRF;
- compatibility migration/link from existing `webmaster_users` without losing sites/metrics/billing ownership;
- service navigation: Search / Maps / Mail / Webmaster / account;
- `/webmaster` registration/login/onboarding;
- add site and normalize origin/host;
- ownership instructions for DNS TXT, HTML file and META tag;
- verification check/refresh and expiry/reissue flows;
- verified-site overview;
- sitemap add/status/error UI;
- URL Submit/Reindex/Delete UI;
- index status, crawl diagnostics, robots/canonical/404/500 views;
- impressions, clicks, CTR and Answer citations with bounded date ranges;
- Webmaster Pro entitlement visibility without changing organic ranking;
- tenant isolation/security tests.

### DoD
A new user can create an account, add a domain, receive exact verification instructions, prove ownership, submit sitemap/URLs and view first-party Search/Webmaster analytics without operator help.

## Sprint 23 — Owner Admin Console

### Goal
Turn the existing secure Admin backend/operator page into a complete owner control plane without requiring direct PostgreSQL access.

### Scope
- dashboard and health/resource overview;
- crawler domains/URLs/queues/history;
- index/outbox/rebuild status;
- Search Quality/golden-query reports;
- Demand/Query Gap diagnostics and suppress/reopen;
- Answer Engine diagnostics;
- spam/domain policy controls;
- organizations/imports/review queue;
- maps/address/data versions;
- Webmaster users/sites/verification/support diagnostics;
- users/sessions/security events;
- billing/invoice/reconciliation diagnostics;
- Data Hub publication/suppression/rollback;
- releases, backup/restore readiness and capacity snapshots;
- RBAC for viewer/analyst/operator/superadmin;
- preview/apply + CSRF + immutable audit for critical mutations.

### DoD
The owner can diagnose and operate all critical subsystems through Admin while destructive actions remain permissioned, previewed and auditable.

## Sprint 24 — Maps Product + Reviews

### Goal
Deliver a consumer Maps experience on the existing OSM/PMTiles/MapLibre/GEO stack and add safe first-party organization reviews.

### Scope
- Yandex-Maps-like layout: search/sidebar/map/card, responsive mobile-first;
- GEO/address autocomplete and nearby/category filters;
- organization card: name, category, address, contacts, website, hours when canonical evidence exists;
- canonical rating aggregate independent of source ratings;
- authenticated first-party reviews;
- one active review per user/place with revision history;
- owner replies for verified claimed organizations;
- report/moderation lifecycle;
- abuse/rate limits and rating recomputation excluding hidden/deleted reviews;
- map reviews never modify organic paid placement/ranking;
- stored-XSS and tenant-isolation tests.

## Sprint 25 — Mail Core

### Goal
Create a secure first-party mailbox and internal message system tied to canonical consumer accounts.

### Scope
- mailboxes/addresses;
- Inbox/Sent/Drafts/Trash/Spam folders;
- messages, recipients, threads, read/star flags;
- compose/reply/forward;
- internal user-to-user delivery transaction;
- plain text + sanitized HTML representation;
- attachment metadata/content hash/quotas;
- bounded local attachment blob storage with opaque IDs and non-executable delivery headers;
- no attachment filename is used as a filesystem path;
- message search using PostgreSQL FTS unless evidence justifies another component;
- send/recipient/storage rate limits;
- delete/restore and retention semantics;
- strict mailbox tenant isolation tests.

### Boundary
Internet SMTP/IMAP delivery is not silently bundled into Mail Core. Internal Mail ships first because external mail introduces reputation, relay, bounce, DKIM/SPF/DMARC and abuse requirements.

## Sprint 26 — Internet Mail Gateway

### Goal
Add external email delivery/receiving only after an ADR and abuse model.

### Scope
- ADR for SMTP ingress/egress architecture;
- domain/address policy;
- SPF, DKIM and DMARC;
- outbound queue, retries and bounce processing;
- inbound MIME parser with hard size/part/depth limits;
- spam/abuse throttling and sender reputation controls;
- quarantine workflow;
- external attachment security;
- operator diagnostics.

### Forbidden
Open relay, unrestricted anonymous sending, direct exposure of internal storage paths, automatic execution/rendering of unsafe attachment types.

## Sprint 27 — Security & Commercial Readiness Gate

### Goal
Run a final defensive red-team style audit and commercial-launch gate across all products.

### Scope
- authentication/authorization/IDOR matrix;
- CSRF/CORS/Origin review;
- stored/reflected XSS review;
- SSRF/DNS rebinding/redirect tests;
- SQL/Manticore injection review;
- abuse/rate-limit review for Search, Webmaster, Maps reviews and Mail;
- upload/MIME/archive-bomb/filename/path traversal tests;
- billing replay/isolation regression;
- backup/restore disaster drill;
- index consistency/rebuild drill;
- capacity snapshot after added products;
- dependency/config/secret exposure review;
- production runbooks and launch checklist.

### DoD
No known P0/P1 findings, runtime tests pass, recovery is demonstrated, capacity is measured, and owner-facing operations do not require unsafe direct DB intervention.
