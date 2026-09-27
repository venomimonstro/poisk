# Sprint 28 — Security & Commercial Readiness Gate

**Status:** CODE/STATIC PASS — COMMERCIAL READY: NO (runtime evidence pending)

## Scope completed

Final code/static audit covered public/Next/backend proxy boundaries, consumer/Admin authentication, CSRF/RBAC, tenant isolation, crawler/Webmaster SSRF, Reviews/Maps, Mail attachments/gateway/MTA boundaries, secrets/diagnostics, migration safety, Capacity/Quality/Recovery evidence and the release-readiness decision process.

## P0/P1 findings fixed during Sprint 28

- Bounded request bodies were enforced on Account/Admin/Webmaster/Reviews/Mail Next proxies; route allowlists remain explicit and there is no generic internal backend tunnel.
- Outbound MTA submission no longer follows HTTP redirects. A signed mail envelope and gateway HMAC headers cannot be forwarded to a redirect target.
- Internet Mail application boundary was re-audited: outbound `From` comes from the sender mailbox ACTIVE primary alias, inbound RCPT resolves only to ACTIVE local aliases, gateway requests are HMAC/timestamp/replay protected and inbound delivery is idempotent by event/body/recipient identity.
- Browser transport is fail-closed: every environment other than dev/development/local/test requires `PUBLIC_BASE_URL=https://...`; browser auth cookies use one shared Secure-cookie policy.
- Admin creation no longer accepts the password in argv. The password is read as one bounded stdin line to avoid process-list/shell-history disclosure.
- Mail attachment filenames reject path separators, NUL and all control characters. Blob storage uses server-generated UUID storage keys, tenant-scoped lookup and safe attachment response headers.
- Mail DNS launch evidence is bound to the configured `MAIL_DOMAIN + MAIL_DKIM_SELECTOR`; a readiness snapshot for a previous domain/selector cannot satisfy the gate.
- Exact migration-set validation was added. Missing migrations in the middle of the chain are blockers even if the maximum schema version looks current.
- Release evidence is bound to `RELEASE_VERSION`; the release registry manifest must use the same build SHA, require the current schema and have completed preflight. Only STAGED/ACTIVE can qualify; PREVIOUS cannot certify a new launch.
- **Capacity false-PASS fixed:** the benchmark now records exact `git_commit + database_schema`; readiness accepts only an `ISOLATED_1M` snapshot from the exact candidate build, with >=1,000,000 documents, an ADR, freshness and **no HIGH bottlenecks**. An old or overloaded benchmark cannot certify a new build.
- **Search Quality false-PASS fixed:** quality runs now persist exact `git_commit + database_schema`; readiness accepts only a fresh PASS from the exact candidate build. Historical unbound quality runs remain readable but cannot certify launch.
- **Recovery false-PASS fixed:** a PASS drill must contain artifact reference, SHA-256, positive size and positive duration. Recovery evidence is now also bound to exact candidate git commit + database schema. Historical unverifiable/unbound drills remain history but cannot satisfy readiness.
- Recovery integrity migration is upgrade-safe via `NOT VALID`: historical pre-invariant rows do not break upgrade, while PostgreSQL enforces the rule for new/updated rows and readiness rejects invalid history.
- Environment-sensitive evidence is no longer immortal: BROWSER_SMOKE and SECURITY_REGRESSION expire after 7 days; EDGE_TLS_PROXY and MTA_FLOW expire after 24 hours. Build/unit/integration/fresh-install/upgrade evidence remains immutable for the exact commit/schema.
- A transient indexer wiring regression introduced while adding readiness wiring was detected during the same audit and fixed; Manticore indexing uses its HTTP endpoint again.
- A readiness checksum-validator symbol collision introduced during hardening was caught during static review and fixed before reporting code/static PASS.

No known unresolved **code/static** P0/P1 security or data-integrity defect remains at the time of this report. This statement is not runtime evidence.

## Confirmed boundaries

- Consumer sessions: HttpOnly + SameSite=Strict; Secure outside dev/development/local/test; server-side revocation and caps.
- Admin: separate auth, 2FA, role enforcement, CSRF, session-bound single-use preview/apply for destructive operations.
- Crawler/Webmaster: redirect targets are revalidated; DNS/IP is resolved/validated again at dial time; private, loopback, link-local, metadata and unsafe ports are rejected.
- Mail machine boundary: bounded request bodies, HMAC timestamp/signature validation, replay claim, private internal gateway path, no public backend host port in standard Compose.
- Internet Mail outbound sender is server-derived from the sender mailbox primary alias; arbitrary client-supplied external sender is not used by MTA submission.
- Inbound recipient acceptance resolves only known ACTIVE aliases/mailboxes; repeated inbound events are body/recipient checked and idempotent.
- Attachments: bounded upload, quota/rate limit, tenant-scoped access, UUID storage key, no user-selected server path, safe download headers.
- Admin support diagnostics omit password hashes, raw session/CSRF/token hashes and raw payment payloads.
- Operator CLI does not accept the Admin password or raw provider payment payload in argv.
- `.github/workflows` remains absent; no GitHub Actions/CI was added.

## Commercial readiness infrastructure

`commercial_readiness_evidence` is immutable and records PASS/FAIL evidence against the exact Git commit and database schema with external artifact reference + SHA-256.

Required evidence types:

- BUILD_UNIT
- INTEGRATION
- FRESH_INSTALL
- UPGRADE
- BROWSER_SMOKE — max age 7 days
- SECURITY_REGRESSION — max age 7 days
- EDGE_TLS_PROXY — max age 24 hours
- MTA_FLOW — max age 24 hours when Internet Mail is enabled

`readinessctl check` additionally requires:

- `RELEASE_VERSION` resolves to a STAGED/ACTIVE release whose `build_sha` equals `READINESS_GIT_SHA`, required schema equals the repository schema and preflight completed;
- exact repository/applied migration-set match;
- fresh PASS Search Quality run for the exact commit/schema;
- real exact-build `ISOLATED_1M` Capacity snapshot with >=1,000,000 measured documents, ADR and no HIGH bottlenecks;
- fresh exact-build/current-schema BACKUP and RESTORE PASS drills with verifiable artifact SHA-256, size and duration;
- fresh non-CRITICAL resource-pressure state;
- when Internet Mail is enabled: fresh exact-build MTA_FLOW evidence and fresh DNS readiness for the configured domain/selector with no drift.

The gate exits non-zero while any requirement is missing, stale or failing.

## Tests added/strengthened

- MTA signed request cannot follow 307 redirect to another server.
- MTA URL rejects credentials/userinfo.
- non-local deployment environments require HTTPS public origin.
- readiness migration-set diff and evidence type allowlist.
- Capacity benchmark refuses missing/malformed exact release SHA.
- readiness evidence freshness policy is unit-locked.
- Admin password stdin parser rejects missing/short/multiline input.
- attachment filename traversal/control-character cases.

Existing integration suites cover tenant isolation and security behavior across Webmaster, Mail, Reviews, Billing/Claims and Admin preview/apply paths. These suites still require an actual migrated PostgreSQL run before launch.

## Migration additions in this hardening pass

- `000065_quality_release_binding.sql`
- `000066_recovery_pass_integrity.sql`
- `000067_recovery_release_binding.sql`

Migration versions remain unique; the migration runner also fail-fasts on duplicate versions.

## Operational documentation

`docs/runbooks/security-commercial-readiness.md` defines trust boundaries, required runtime evidence, release-manifest binding, browser/security smoke matrices, launch-gate sequence, incident procedures and the rule that missing evidence means NOT READY.

## Runtime evidence status

Not claimed. The current development execution environment did not provide a trustworthy full runtime stack for all launch gates.

Still required before commercial launch:

1. stage/preflight the **exact candidate release** and bind `RELEASE_VERSION + READINESS_GIT_SHA`;
2. backend build + complete Go unit test run and frontend build/type check for that exact commit;
3. complete integration/security suite on migrated PostgreSQL;
4. empty-database fresh install to the exact current migration set;
5. supported previous-schema upgrade with retained canonical data;
6. browser smoke through the real HTTPS edge for Search, Account, Webmaster, Maps/Reviews, Mail and Admin;
7. fresh SECURITY_REGRESSION evidence;
8. fresh EDGE_TLS_PROXY evidence;
9. fresh exact-build Search Quality PASS;
10. real exact-build 1M Capacity run + ADR with no HIGH bottlenecks;
11. exact-build/current-schema verifiable BACKUP PASS and RESTORE PASS drills;
12. if Internet Mail is enabled: fresh exact-build MTA flow plus configured-domain/selector DNS readiness with no drift.

## Launch verdict

**NOT READY for commercial production yet.**

The remaining blocker is no longer missing product architecture. It is the absence of required real runtime evidence for the exact release manifest/commit/schema. Commercial READY may be declared only after those artifacts are recorded and `readinessctl check` returns `ready=true`.
