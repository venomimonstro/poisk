# Security & Commercial Readiness Runbook

## Purpose

This runbook is the release gate for a commercial deployment. A code/static review is not production evidence. A release is **NOT READY** until `readinessctl check` returns `ready=true` for the exact Git commit and exact migration set deployed.

## Trust boundaries

- Public traffic terminates TLS before the bundled Nginx. `PUBLIC_BASE_URL` must be an HTTPS origin for every environment except `dev`, `development`, `local`, and `test`.
- Nginx is the only public container port in the standard Compose deployment. PostgreSQL, Manticore, backend and internal mail gateway must not be host-published.
- Browser sessions use HttpOnly + SameSite=Strict cookies. Secure cookies are mandatory outside local/dev/test.
- Browser mutations require CSRF. Machine mail callbacks use bounded HMAC authentication, timestamp validation and replay protection.
- Crawler/Webmaster fetches only use HTTP(S) ports 80/443, revalidate every redirect and re-resolve/validate DNS again at dial time. Private, loopback, link-local and metadata ranges are rejected.
- Mail attachments are stored under the configured blob root with validated storage keys. Downloads are attachments with `nosniff`, private/no-store caching and CSP sandbox.
- Internet Mail is disabled by default. The application submits only to the configured HTTPS MTA origin and never follows MTA redirects.

## Required runtime evidence

Set `READINESS_GIT_SHA` to the exact 40-character commit under test. Evidence must be for the current database schema and this exact commit.

Required evidence types:

- `BUILD_UNIT` — backend build, Go unit tests, frontend build/type checks.
- `INTEGRATION` — integration tests against migrated PostgreSQL, including tenant-isolation/security suites.
- `FRESH_INSTALL` — empty database migrated to the exact repository migration set and service boot smoke.
- `UPGRADE` — supported previous schema upgraded to the exact repository migration set without data loss.
- `BROWSER_SMOKE` — Search, Account, Webmaster, Maps/Reviews, Mail and Admin browser paths.
- `SECURITY_REGRESSION` — auth/session/CSRF/RBAC/tenant isolation, SSRF, path traversal, body limits, mail HMAC/replay and MTA redirect regression suite.
- `EDGE_TLS_PROXY` — real HTTPS edge test proving redirect-to-HTTPS policy, secure cookies and correct proxy boundary.
- `MTA_FLOW` — required only when Internet Mail is enabled; proves outbound submit, delivery/bounce callback, inbound delivery and replay rejection.

Each evidence artifact must have a SHA-256 digest. Record only a reference/path and digest; do not store passwords, session tokens, gateway secrets, DKIM private keys, payment payloads or raw MIME in readiness evidence.

Example shape (replace placeholders with real values):

```text
READINESS_GIT_SHA=<exact-commit> ./app readinessctl record BUILD_UNIT PASS <artifact-ref> <sha256> <actor>
```

Repeat for every required evidence type. A FAIL record is retained immutably and a later successful rerun must create a new PASS record; evidence is never edited or deleted.

## Release-gate sequence

1. Deploy/test the exact candidate commit in an isolated release environment.
2. Run migrations. Confirm the applied migration version set exactly equals the repository version set; missing versions in the middle are blockers.
3. Produce and record `BUILD_UNIT` evidence.
4. Produce and record `INTEGRATION` evidence.
5. Verify a fresh installation and record `FRESH_INSTALL` evidence.
6. Restore a supported previous-schema fixture, upgrade it, validate retained canonical data and record `UPGRADE` evidence.
7. Run browser smoke for Search, Account, Webmaster, Maps/Reviews, Mail and Admin; record `BROWSER_SMOKE`.
8. Run the security regression matrix; record `SECURITY_REGRESSION`.
9. Verify the real HTTPS edge and secure-cookie behavior; record `EDGE_TLS_PROXY`.
10. Run the Search Quality gate. The latest run must PASS and be no older than 7 days.
11. Run an `ISOLATED_1M` Capacity benchmark with at least 1,000,000 measured documents, real server CPU/RAM/disk measurements and an ADR. It must be no older than 30 days.
12. Run BACKUP and RESTORE drills against the current schema. Both latest drills must PASS and be no older than 30 days.
13. Confirm `resource_pressure` is fresh and not CRITICAL.
14. If Internet Mail is enabled: verify DNS readiness with no drift, run the real MTA flow, and record `MTA_FLOW`.
15. Run:

```text
READINESS_GIT_SHA=<exact-commit> ./app readinessctl check
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
- Files: storage keys cannot escape blob/map/import roots; user filenames never select server paths.
- Mail machine boundary: HMAC signature, bounded clock skew, replay rejection, bounded bodies, no MTA redirects.
- Privacy/logging: routine diagnostics do not expose password hashes, session/CSRF/token hashes, raw gateway secrets, DKIM private keys, raw payment event payloads, raw MIME, or full recipient lists.

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
3. Use the latest verified current-schema backup and the recovery procedure. A backup without a successful restore drill is not considered sufficient evidence.
4. After restore, verify migration set, canonical counts, index consistency and Search/Map/Webmaster/Mail smoke before reopening writes.
5. Record new recovery and readiness evidence; never reuse evidence from a different commit/schema.

### Capacity incident

1. Inspect `resource_pressure`, crawl/outbox backlogs, PostgreSQL/Manticore document-count mismatch, disk availability and P95/P99.
2. Heavy workers must shed/stop under CRITICAL pressure; do not disable Search safeguards to catch up queues.
3. Any architectural scaling change follows the Capacity ADR process. Do not introduce a new queue/database/platform ad hoc during an incident.

## Launch verdict

If any required evidence is missing, stale, FAIL, tied to another commit/schema, or if any known P0/P1 remains unresolved, the verdict is **NOT READY**. Missing runtime evidence must never be converted into a code/static PASS.
