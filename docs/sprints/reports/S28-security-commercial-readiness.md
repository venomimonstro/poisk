# Sprint 28 — Security & Commercial Readiness Gate

**Status:** CODE/STATIC PASS — COMMERCIAL READY: NO (runtime evidence pending)

## Scope completed

Final code/static audit covered public/Next/backend proxy boundaries, consumer/Admin authentication, CSRF/RBAC, tenant isolation, crawler/Webmaster SSRF, Mail attachments/gateway/MTA boundaries, secrets/diagnostics, migration safety and the release-readiness decision process.

## P0/P1 findings fixed during Sprint 28

- Bounded request bodies were enforced on Account/Admin/Webmaster/Reviews Next proxies; route allowlists remain explicit and there is no generic internal backend tunnel.
- Outbound MTA submission no longer follows HTTP redirects. A signed mail envelope and gateway HMAC headers cannot be forwarded to a redirect target.
- Production-like browser transport is fail-closed: every environment other than dev/development/local/test requires `PUBLIC_BASE_URL=https://...`; browser auth cookies use one shared Secure-cookie policy.
- Admin creation no longer accepts the password in argv. The password is read as one bounded stdin line to avoid process-list/shell-history disclosure.
- Mail attachment filenames reject path separators, NUL and all control characters. Blob storage continues to use server-generated UUID storage keys, tenant-scoped lookup and safe attachment response headers.
- Mail DNS launch evidence is now bound to the currently configured `MAIL_DOMAIN + MAIL_DKIM_SELECTOR`; a readiness snapshot for a previous domain/selector cannot satisfy the gate.
- Exact migration-set validation was added to readiness. Missing migrations in the middle of the chain are blockers even if the maximum schema version looks current.
- A transient indexer wiring regression introduced while adding readiness wiring was detected in the same audit and fixed immediately; Manticore indexing uses its HTTP endpoint again.

No known unresolved P0/P1 security or data-integrity defect remains in the code/static audit at the time of this report. This statement is not a substitute for runtime security/integration evidence.

## Confirmed boundaries

- Consumer sessions: HttpOnly + SameSite=Strict; Secure outside dev/development/local/test; server-side revocation and caps.
- Admin: separate auth, 2FA, role enforcement, CSRF, session-bound single-use preview/apply for destructive operations.
- Crawler/Webmaster: redirect targets are revalidated; DNS/IP is resolved/validated again at dial time; private, loopback, link-local, metadata and unsafe ports are rejected.
- Mail machine boundary: bounded request bodies, HMAC timestamp/signature validation, replay claim, private internal gateway path, no public backend host port in standard Compose.
- Attachments: bounded upload, quota/rate limit, tenant-scoped access, UUID storage key, no user-selected server path, safe download headers.
- Admin support diagnostics omit password hashes, raw session/CSRF/token hashes and raw payment payloads.
- `.github/workflows` is absent; no GitHub Actions/CI was added.

## Commercial readiness infrastructure added

`commercial_readiness_evidence` is immutable and records PASS/FAIL evidence against the exact Git commit and database schema with an external artifact reference + SHA-256.

Required evidence types:

- BUILD_UNIT
- INTEGRATION
- FRESH_INSTALL
- UPGRADE
- BROWSER_SMOKE
- SECURITY_REGRESSION
- EDGE_TLS_PROXY
- MTA_FLOW when Internet Mail is enabled

`readinessctl check` additionally requires:

- exact repository/applied migration-set match;
- recent PASS Search Quality gate;
- real `ISOLATED_1M` Capacity snapshot with >=1,000,000 measured documents and an ADR;
- fresh current-schema BACKUP and RESTORE PASS drills;
- fresh non-CRITICAL resource-pressure state;
- when Internet Mail is enabled: MTA_FLOW evidence and fresh DNS readiness for the configured domain/selector with no drift.

The gate exits non-zero while any requirement is missing or failing.

## Tests added/strengthened

- MTA signed request cannot follow 307 redirect to another server.
- MTA URL rejects credentials/userinfo.
- non-local deployment environments require HTTPS public origin.
- readiness migration-set diff and evidence type allowlist.
- Admin password stdin parser rejects missing/short/multiline input.
- attachment filename traversal/control-character cases.

Existing integration suites cover tenant isolation and security behavior across Webmaster, Mail, Reviews, Billing/Claims and Admin preview/apply paths. These suites still require an actual migrated PostgreSQL run before launch.

## Operational documentation

`docs/runbooks/security-commercial-readiness.md` defines:

- trust boundaries;
- required runtime evidence;
- browser smoke matrix;
- security regression matrix;
- exact release-gate sequence;
- account/SSRF/mail/data/capacity incident procedures;
- the rule that missing evidence always means NOT READY.

## Runtime evidence status

Not claimed in this report. The current execution environment used for development did not provide a trustworthy full runtime stack for all of the following gates.

Still required before commercial launch:

1. backend build + complete Go unit test run and frontend build/type check for the exact candidate commit;
2. complete integration/security suite on migrated PostgreSQL;
3. empty-database fresh install to the exact current migration set;
4. supported previous-schema upgrade with retained canonical data;
5. browser smoke through the real HTTPS edge for Search, Account, Webmaster, Maps/Reviews, Mail and Admin;
6. SECURITY_REGRESSION evidence;
7. EDGE_TLS_PROXY evidence;
8. fresh Search Quality PASS;
9. real 1M Capacity run + ADR;
10. current-schema BACKUP PASS and RESTORE PASS drill;
11. if Internet Mail is enabled: real MTA flow plus fresh configured-domain/selector DNS readiness with no drift.

## Launch verdict

**NOT READY for commercial production yet.**

The remaining blocker is no longer missing product architecture: it is the absence of the required real runtime evidence for the exact release commit/schema. Commercial READY may be declared only after those artifacts are recorded and `readinessctl check` returns `ready=true`.
