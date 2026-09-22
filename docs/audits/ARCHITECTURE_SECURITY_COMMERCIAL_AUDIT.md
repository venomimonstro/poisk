# Architecture, Security & Commercial Readiness Audit

**Status:** ACTIVE AUDIT  
**Commercial readiness:** NOT READY FOR COMMERCIAL LAUNCH  
**Current active sprint:** 21 — Capacity Gate

## 1. Executive conclusion

The project has a strong production-minded backend foundation: PostgreSQL is canonical, Manticore is rebuildable, index writes use versioned outbox semantics, crawler input is bounded, Admin has separate RBAC/2FA/CSRF, resource pressure can stop heavy workers, billing is idempotent, and Data Hub publication is evidence-gated.

However, the system must not be declared commercially ready yet. The blockers below are launch gates, not cosmetic backlog items.

## 2. Blocking findings

### P0 — build/capacity correctness

1. `cmd/app/capacity_manticore.go` called `manticore.Client.CountDocuments()` while the method was missing. This was a compile blocker in the Capacity Gate path. Fixed during this audit.
2. Capacity classification had an `INDEX_COUNT_MISMATCH` rule, but `capacityctl benchmark` did not populate PostgreSQL and Manticore document counts. A desynchronized search index could therefore escape the gate. Fixed during this audit.
3. Full repository build, migration upgrade, integration and runtime test execution is still not proven in the current tool environment. Static/code gates must not be represented as runtime PASS.
4. Sprint 21 still requires a real production-like benchmark, including an isolated >=1M indexed-document run and a separately labelled 10M projection. Projected values are not measured values.

### P1 — product architecture

1. **Identity is fragmented.** Webmaster users/sessions and Admin identities are separate systems. This is acceptable for Admin isolation, but consumer-facing products (Maps reviews, Mail, account settings, cross-service navigation) need one canonical user identity/session layer. Mail must not be built directly on Webmaster bearer sessions.
2. **Webmaster has backend capability without a complete end-user cabinet.** Site ownership, verification, sitemap, URL submission and metrics exist in backend schema/services, but there is no complete `/webmaster` product UI comparable to a usable Webmaster console.
3. **Owner Admin is incomplete as a product surface.** `/admin` currently covers authentication, operational status and domain policy preview/apply. The owner still needs crawler, URLs, index, Search Quality, Query Gaps, Answer Engine, spam, organizations, maps, Webmaster, users, billing, releases/backups and system controls in one RBAC-safe console.
4. **Maps lack first-party review lifecycle.** GEO/OSM/PMTiles/MapLibre and organization cards exist, but user reviews, rating aggregates, moderation, reports and owner replies are not implemented.
5. **Mail is absent.** A Yandex-Mail-like product needs canonical accounts, mailbox/message isolation, quotas, attachment security, anti-abuse and delivery architecture. It cannot safely be added as a UI-only feature.
6. **Commercial gate is incomplete.** Real capacity measurements, recovery drill, attack/abuse regression, alerting/operational runbook validation and product-surface completeness are required before accepting paying users.

## 3. Architecture assessment

### Correct decisions to retain

- PostgreSQL is the canonical source of truth.
- Manticore contains rebuildable search representations, not business truth.
- Versioned outbox prevents old index events from overwriting newer canonical state.
- Crawler has SSRF/DNS/IP/redirect checks and bounded response/sitemap processing.
- Queue workers use bounded leases/idempotency patterns.
- Heavy workers react to resource-pressure watermarks.
- Admin is intentionally separated from ordinary user auth and requires 2FA/RBAC/CSRF.
- Destructive Admin actions use preview/apply and audit instead of direct one-click mutation.
- Billing uses integer money, immutable events and idempotent provider event handling.
- Paid products do not change organic ranking or Demand Driven crawl priority.
- Data Hub pages are versioned, evidence-gated, suppressible and rebuildable.

### Architecture changes required before Mail/reviews

Introduce a canonical **consumer account** layer. Do not merge Admin accounts into it. Webmaster identities must be linked/migrated compatibly to the consumer identity rather than duplicated indefinitely.

Target relationship:

```text
consumer_users
  ├─ user_sessions
  ├─ webmaster_profile / owned sites
  ├─ organization reviews
  ├─ claimed organizations
  ├─ mailboxes
  └─ billing ownership

admin_users (separate security boundary)
```

## 4. Defensive security review / attack surface

The following scenarios must have explicit tests or controls before commercial launch.

### Authentication / authorization

- IDOR between Webmaster sites, agencies, organization claims, reviews and mailboxes.
- Session fixation, replay and stolen-session revocation.
- Password reset/email verification token replay and expiration.
- CSRF on all cookie-authenticated mutation endpoints.
- 2FA bypass/recovery-code reuse for Admin.
- privilege escalation between Admin roles and Agency roles.
- tenant-isolation queries must bind both resource ID and authenticated owner/member.

### Web / API

- stored/reflected XSS through organization names, reviews, snippets, mail subjects/bodies and attachment filenames.
- unsafe CORS/Origin reflection.
- forged reverse-proxy IP headers bypassing rate limits.
- request-body bombs, excessive JSON nesting and oversized query/filter values.
- SQL/Manticore query injection where query strings are assembled dynamically.
- open redirect and URL-scheme injection.
- public endpoints that trigger unbounded DB writes.

### Crawler / external fetches

- DNS rebinding and redirect-to-private-IP SSRF.
- decompression bombs and content-length mismatch.
- sitemap recursion/URL explosion.
- canonical poisoning and hostile Schema.org payloads.
- path/query traps and unbounded calendars/search/filter pages.

### Reviews

- rating/review flooding by one account.
- duplicate account abuse and owner self-review abuse signals.
- stored XSS/HTML injection in reviews and owner replies.
- moderation/report race conditions.
- deleted/hidden review still affecting rating aggregate.

### Mail / attachments

- cross-mailbox IDOR is a P0 class issue.
- HTML mail must be sanitized; scripts/forms/unsafe URLs cannot execute.
- attachment filenames must be normalized and never become filesystem paths.
- MIME type must be sniffed server-side; browser-provided Content-Type is not trusted.
- archive/decompression bombs require hard unpacking limits; automatic archive extraction is disabled by default.
- executable/SVG/HTML attachment inline rendering must be restricted.
- attachment bytes need content hash, quota and bounded size.
- rate limits are required for compose/send, recipients/day and attachment bytes/day.
- external SMTP, if added, requires SPF/DKIM/DMARC, bounce handling and abuse controls before public availability.

### Operations

- backup archives must not contain runtime secrets.
- restore must require explicit destructive confirmation and a tested recovery run.
- release activation occurs only after candidate health checks.
- Search remains available under resource pressure while new heavy leases are stopped.
- PostgreSQL↔Manticore count/version consistency must be part of operational diagnostics.

## 5. Commercial launch gate

Commercial status can change to READY only when all of the following are true:

- clean build of backend and frontend;
- migrations pass on empty DB and previous-version upgrade path;
- unit/integration/security tests pass;
- restore drill passes from a real backup;
- Capacity Gate has measured Search/GEO/address figures and a documented 1M run;
- PostgreSQL/Manticore index consistency is within tolerance;
- no open P0/P1 audit findings;
- canonical consumer identity is deployed;
- Webmaster cabinet is complete enough for self-service site onboarding and ownership verification;
- owner Admin covers critical operations without direct DB access;
- reviews have moderation/abuse controls before public write access;
- Mail passes tenant-isolation and attachment-security tests before public write access;
- monitoring and operator runbooks identify queue, index, disk, memory, auth and abuse failures.

## 6. Audit policy

This document is updated as P0/P1 findings are discovered. A finding is closed only by code/tests or a documented architectural decision; hiding the finding or relabelling an unexecuted test as PASS is forbidden.
