# Security & Commercial Readiness Runbook

## Purpose

This runbook is the release gate for a commercial deployment. A code/static review is not production evidence. A release is **NOT READY** until `readinessctl check` returns `ready=true` for the exact staged release version, Git commit and migration set deployed.

## Trust boundaries

- Public traffic terminates TLS before the bundled Nginx. `PUBLIC_BASE_URL` must be an HTTPS origin for every environment except `dev`, `development`, `local`, and `test`.
- Nginx is the only public container port in the standard Compose deployment. PostgreSQL, Manticore, backend and internal mail gateway must not be host-published.
- Browser sessions use HttpOnly + SameSite=Strict cookies. Secure cookies are mandatory outside local/dev/test.
- Browser mutations require CSRF. Machine mail callbacks use bounded HMAC authentication, timestamp validation and replay protection.
- Crawler/Webmaster fetches only use HTTP(S) ports 80/443, revalidate every redirect and re-resolve/validate DNS again at dial time. Private, loopback, link-local and metadata ranges are rejected.
- Mail attachments are stored under the configured blob root with validated storage keys. Downloads are attachments with `nosniff`, private/no-store caching and CSP sandbox.
- Internet Mail is disabled by default. The application submits only to the configured HTTP(S) MTA origin and never follows MTA redirects. Public SMTP relay policy belongs to the MTA edge and must be proven by runtime evidence; the application does not accept arbitrary client-supplied outbound envelope senders.

## Required runtime evidence

Set `READINESS_GIT_SHA` to the exact 40-character commit under test and `RELEASE_VERSION` to the staged release candidate. The release registry entry must reference the same build SHA, require the current schema and have completed preflight. Evidence must be for the current database schema and this exact commit.

Required evidence types:

- `BUILD_UNIT` — backend build, Go unit tests, frontend build/type checks.
- `INTEGRATION` — integration tests against migrated PostgreSQL, including tenant-isolation/security suites.
- `FRESH_INSTALL` — empty database migrated to the exact repository migration set and service boot smoke.
- `UPGRADE` — supported previous schema upgraded to the exact repository migration set without data loss.
- `BROWSER_SMOKE` — Search, Account, Webmaster, Maps/Reviews, Mail and Admin browser paths; max age 7 days.
- `SECURITY_REGRESSION` — auth/session/CSRF/RBAC/tenant isolation, SSRF, path traversal, body limits, mail HMAC/replay and MTA abuse regression suite; max age 7 days.
- `EDGE_TLS_PROXY` — real HTTPS edge test proving redirect-to-HTTPS policy, secure cookies and correct proxy boundary; max age 24 hours.
- `MTA_FLOW` — required only when Internet Mail is enabled; proves outbound submit, delivery/bounce callback, inbound delivery, replay rejection and SMTP relay/sender policy; max age 24 hours.

Each evidence artifact must have a SHA-256 digest. Record only a reference/path and digest; do not store passwords, session tokens, gateway secrets, DKIM private keys, payment payloads or raw MIME in readiness evidence.

Example shape (replace placeholders with real values):

```text
READINESS_GIT_SHA=<exact-commit> RELEASE_VERSION=<staged-version> ./app readinessctl record BUILD_UNIT PASS <artifact-ref> <sha256> <actor>
```

Repeat for every required evidence type. A FAIL record is retained immutably and a later successful rerun must create a new PASS record; evidence is never edited or deleted.

## Release-gate sequence

1. Stage the exact candidate in the release registry and run release preflight.
2. Deploy/test that exact candidate commit in an isolated release environment.
3. Run migrations. Confirm the applied migration version set exactly equals the repository version set; missing versions in the middle are blockers.
4. Produce and record `BUILD_UNIT` evidence.
5. Produce and record `INTEGRATION` evidence.
6. Verify a fresh installation and record `FRESH_INSTALL` evidence.
7. Restore a supported previous-schema fixture, upgrade it, validate retained canonical data and record `UPGRADE` evidence.
8. Run browser smoke for Search, Account, Webmaster, Maps/Reviews, Mail and Admin; record `BROWSER_SMOKE`.
9. Run the security regression matrix; record `SECURITY_REGRESSION`.
10. Verify the real HTTPS edge and secure-cookie behavior; record `EDGE_TLS_PROXY`.
11. Run the Search Quality gate with the same `READINESS_GIT_SHA`. The stored quality run must belong to the exact candidate commit/schema, PASS and be no older than 7 days.
12. Run an `ISOLATED_1M` Capacity benchmark with the same `READINESS_GIT_SHA`, at least 1,000,000 measured documents, real server CPU/RAM/disk measurements and an ADR. The snapshot must belong to the exact candidate commit/schema, contain no HIGH bottlenecks and be no older than 30 days.
13. Run BACKUP and RESTORE drills with the same `READINESS_GIT_SHA`. Both drills must belong to the exact candidate commit/schema, include artifact ref + SHA-256 + positive byte size + positive duration, PASS and be no older than 30 days.
14. Confirm `resource_pressure` is fresh and not CRITICAL.
15. If Internet Mail is enabled: verify DNS readiness for the currently configured `MAIL_DOMAIN + MAIL_DKIM_SELECTOR` with no drift, run the full MTA abuse/delivery matrix below, and record fresh `MTA_FLOW` evidence.
16. Run:

```text
READINESS_GIT_SHA=<exact-commit> RELEASE_VERSION=<staged-version> ./app readinessctl check
```

Only `ready=true` permits a commercial release.

## Browser smoke matrix

- Search: home page, query, pagination/result click, Answer fallback/available paths.
- Account: register/login, CSRF rotation, sessions list, revoke current/other session, logout.
- Webmaster: add site, ownership proof instructions, verify, sitemap submit, URL submit/delete, metrics and URL diagnostics; verify another tenant cannot access the site.
- Maps/Reviews: viewport/search, company card, rating/list, create/edit/delete own review, report another review, verified-owner reply, moderation visibility.
- Mail: mailbox, draft, send internal mail, sent/inbox/trash/search, attachment upload/download, another tenant cannot read item/attachment.
- Admin: login + 2FA, VIEWER/ANALYST read-only behavior, OPERATOR preview/apply single-use semantics, owner/system/support/datahub/organizations/reviews/mail pages.

## Security regression matrix

- Authentication: brute-force lockout, session cap, session revocation, expired/revoked cookie rejection.
- CSRF: every browser mutation rejects missing/wrong CSRF.
- RBAC: VIEWER/ANALYST cannot mutate; SUPERADMIN inheritance is explicit; destructive actions that require confirmation use session-bound preview/apply.
- Tenant isolation: Webmaster site IDs, Mail item/attachment IDs, Reviews ownership/replies, Billing accounts/usage and Organization Claims cannot cross tenants.
- SSRF: private/loopback/link-local/metadata addresses, unsafe ports, URL userinfo and redirect-to-private targets are rejected; DNS is revalidated at dial time.
- Proxy/body limits: every mutating Next proxy has an explicit path allowlist and bounded body; no generic internal backend tunnel exists.
- Files: storage keys cannot escape blob/map/import roots; user filenames never select server paths and attachment names reject control characters.
- Mail machine boundary: HMAC signature, bounded clock skew, replay rejection, bounded bodies, no MTA redirects.
- Internet Mail MTA: unauthenticated external→external SMTP relay is rejected; arbitrary/forged outbound envelope sender is rejected; unknown local RCPT is rejected before message acceptance; known local alias routes only to its mapped mailbox; duplicate inbound event is idempotent; same event ID with changed body/recipient is rejected; forged callback/HMAC is rejected; bounce/suppression path does not recursively generate uncontrolled mail.
- Privacy/logging: routine diagnostics do not expose password hashes, session/CSRF/token hashes, raw gateway secrets, DKIM private keys, raw payment event payloads, raw MIME, or full recipient lists.
- Operator CLI: passwords/raw provider payloads are not passed in argv; secrets use stdin or server-side environment/secret storage as documented.

## Internet Mail runtime matrix

When Internet Mail is enabled, `MTA_FLOW` is not satisfied by a single successful message. The artifact must record all of these outcomes for the exact release:

1. authenticated local mailbox → external recipient: accepted and delivery callback reconciled;
2. external sender → existing local alias: accepted once and visible only in the mapped mailbox;
3. external sender → unknown local alias: rejected;
4. external sender → external recipient through the public SMTP edge: relay rejected;
5. attempt to submit outbound mail with an envelope sender not owned by the authenticated/local mailbox: rejected;
6. repeated gateway callback/event: idempotent/replay rejected as designed;
7. same event identifier with changed signed body or recipient: rejected as mismatch;
8. permanent delivery failure creates suppression/bounce state without an uncontrolled bounce loop;
9. attachment/body size limits are enforced at MTA and application gateway boundaries;
10. DKIM signing, SPF policy and DMARC alignment/readiness match the configured domain/selector snapshot.

## Incident actions

### Suspected account/session compromise

1. Revoke affected consumer/admin sessions server-side.
2. Preserve security/audit events before making unrelated changes.
3. Rotate affected credentials/secrets; never place replacement values in tickets or readiness evidence.
4. If Admin secret material may be exposed, rotate the Admin encryption key through the documented secret-management process and require fresh sessions/2FA setup as appropriate.

### Suspected crawler/Webmaster SSRF abuse

1. Pause/block the affected domain through existing Admin policy controls.
2. Stop heavy crawler leases if required; Search remains available.
3. Preserve target URL, domain policy and security error metadata, but do not log response bodies from sensitive/blocked targets.
4. Verify resolver/dial validation before resuming.

### Mail abuse or compromised MTA boundary

1. Set `MAIL_INTERNET_ENABLED=false` and stop the mail gateway worker if needed. Internal first-party mail can remain independent if safe.
2. Rotate `MAIL_GATEWAY_SHARED_SECRET` on both application and MTA sides.
3. Rotate DKIM private material only at the MTA boundary; never copy it into PostgreSQL or application diagnostics.
4. Inspect immutable delivery events, suppressions and callback replay records.
5. Re-run DNS readiness and full MTA flow before re-enabling Internet Mail.

### Data corruption or failed release

1. Stop writes/heavy workers as appropriate.
2. Use the immutable release registry to select the previous validated release; do not ad-hoc edit release state.
3. Use the latest verified **exact-release** backup and recovery procedure. A backup without a successful restore drill for the same commit/schema is not sufficient evidence.
4. After restore, verify migration set, canonical counts, index consistency and Search/Map/Webmaster/Mail smoke before reopening writes.
5. Record new recovery and readiness evidence; never reuse evidence from a different commit/schema/release manifest.

### Capacity incident

1. Inspect `resource_pressure`, crawl/outbox backlogs, PostgreSQL/Manticore document-count mismatch, disk availability and P95/P99.
2. Heavy workers must shed/stop under CRITICAL pressure; do not disable Search safeguards to catch up queues.
3. Any architectural scaling change follows the Capacity ADR process. Do not introduce a new queue/database/platform ad hoc during an incident.

## Launch verdict

If any required evidence is missing, stale, FAIL, tied to another commit/schema/release manifest, contains a HIGH Capacity bottleneck, or if any known P0/P1 remains unresolved, the verdict is **NOT READY**. Missing runtime evidence must never be converted into a code/static PASS.
