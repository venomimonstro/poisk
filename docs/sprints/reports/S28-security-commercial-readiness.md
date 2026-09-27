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
- Release manifests now require the exact lowercase 40-character Git commit SHA. Arbitrary build labels can no longer create a release candidate that can never satisfy readiness.
- Re-staging a STAGED release invalidates `preflight_at`; changing build/config/images can no longer reuse a previous preflight. A repeated preflight without restage is idempotent and keeps its original timestamp, so activation does not invalidate evidence collected after that preflight.
- Generic commercial evidence is now bound to exact `RELEASE_VERSION + git_commit + database_schema`; historical rows without release binding remain history only and cannot certify a new candidate.
- Deployment-sensitive evidence (`BROWSER_SMOKE`, `SECURITY_REGRESSION`, `EDGE_TLS_PROXY`, `MTA_FLOW`) must also have been completed **after the current release preflight**. Evidence from before a restage/preflight cannot certify the changed deployment.
- **Capacity false-PASS fixed:** benchmark config now records exact `release_version + git_commit + database_schema`; readiness accepts only an `ISOLATED_1M` snapshot for the exact candidate, with >=1,000,000 documents, an ADR, freshness and **no HIGH bottlenecks**.
- **Search Quality false-PASS fixed:** quality runs persist exact `release_version + git_commit + database_schema` plus SHA-256 of the golden set and thresholds. Readiness accepts only a fresh PASS for the exact candidate with both input hashes present. Historical/unbound quality runs cannot certify launch.
- **Recovery false-PASS fixed:** PASS drills require artifact reference, SHA-256 metadata, positive size and positive duration and are now bound to exact `release_version + git_commit + database_schema`. Historical unbound drills cannot satisfy readiness.
- Recovery integrity migrations remain upgrade-safe: historical pre-invariant rows do not become launch evidence, while new candidate-bound drills are validated fail-closed.
- Environment-sensitive evidence is not immortal: BROWSER_SMOKE and SECURITY_REGRESSION expire after 7 days; EDGE_TLS_PROXY and MTA_FLOW expire after 24 hours. Build/unit/integration/fresh-install/upgrade evidence is immutable but still candidate-bound.
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

`commercial_readiness_evidence` is immutable and records PASS/FAIL evidence against the exact release candidate, Git commit and database schema with external artifact reference + SHA-256 metadata.

Required evidence types:

- BUILD_UNIT
- INTEGRATION
- FRESH_INSTALL
- UPGRADE
- BROWSER_SMOKE — max age 7 days and must be after release preflight
- SECURITY_REGRESSION — max age 7 days and must be after release preflight
- EDGE_TLS_PROXY — max age 24 hours and must be after release preflight
- MTA_FLOW — max age 24 hours and must be after release preflight when Internet Mail is enabled

`readinessctl check` additionally requires:

- `RELEASE_VERSION` resolves to a STAGED/ACTIVE release whose exact `build_sha` equals `READINESS_GIT_SHA`, required schema equals the repository schema and preflight completed;
- exact repository/applied migration-set match;
- fresh Search Quality PASS for the exact release/commit/schema with golden + threshold SHA-256 bindings;
- real `ISOLATED_1M` Capacity snapshot for the exact release/commit/schema with >=1,000,000 measured documents, ADR and no HIGH bottlenecks;
- fresh BACKUP and RESTORE PASS drills for the exact release/commit/schema with artifact SHA-256 metadata, positive size and duration;
- fresh non-CRITICAL resource-pressure state;
- when Internet Mail is enabled: fresh exact-candidate MTA_FLOW evidence and fresh DNS readiness for the configured domain/selector with no drift.

The gate exits non-zero while any requirement is missing, stale or failing.

## Tests added/strengthened

- MTA signed request cannot follow 307 redirect to another server.
- MTA URL rejects credentials/userinfo.
- non-local deployment environments require HTTPS public origin.
- readiness migration-set diff and evidence type allowlist.
- deployment evidence post-preflight policy is unit-locked.
- release manifest rejects short/uppercase/non-Git build SHA values.
- Capacity benchmark refuses missing/malformed exact commit and release candidate binding.
- readiness evidence freshness policy is unit-locked.
- Admin password stdin parser rejects missing/short/multiline input.
- attachment filename traversal/control-character cases.

Existing integration suites cover tenant isolation and security behavior across Webmaster, Mail, Reviews, Billing/Claims and Admin preview/apply paths. These suites still require an actual migrated PostgreSQL run before launch.

## Migration additions in this hardening pass

- `000065_quality_release_binding.sql`
- `000066_recovery_pass_integrity.sql`
- `000067_recovery_release_binding.sql`
- `000068_quality_input_binding.sql`
- `000069_readiness_release_binding.sql`
- `000070_recovery_release_binding.sql`
- `000071_quality_candidate_binding.sql`

Migration versions remain unique; the migration runner also fail-fasts on duplicate versions.

## Operational documentation

`docs/runbooks/security-commercial-readiness.md` defines trust boundaries, required runtime evidence, release-manifest binding, browser/security smoke matrices, launch-gate sequence, incident procedures and the rule that missing evidence means NOT READY.

## Runtime evidence status

Not claimed. The current development execution environment did not provide a trustworthy full runtime stack for all launch gates.

Still required before commercial launch:

1. stage/preflight the **exact candidate release** and bind `RELEASE_VERSION + READINESS_GIT_SHA`;
2. backend build + complete Go unit test run and frontend build/type check for that exact candidate;
3. complete integration/security suite on migrated PostgreSQL;
4. empty-database fresh install to the exact current migration set;
5. supported previous-schema upgrade with retained canonical data;
6. browser smoke through the real HTTPS edge for Search, Account, Webmaster, Maps/Reviews, Mail and Admin after candidate preflight;
7. fresh post-preflight SECURITY_REGRESSION evidence;
8. fresh post-preflight EDGE_TLS_PROXY evidence;
9. fresh exact-candidate Search Quality PASS;
10. real exact-candidate 1M Capacity run + ADR with no HIGH bottlenecks;
11. exact-candidate BACKUP PASS and RESTORE PASS drills;
12. if Internet Mail is enabled: fresh post-preflight MTA flow plus configured-domain/selector DNS readiness with no drift.

## Launch verdict

**NOT READY for commercial production yet.**

The remaining blocker is no longer missing product architecture. It is the absence of required real runtime evidence for the exact release manifest/candidate/commit/schema. Commercial READY may be declared only after those artifacts are recorded and `readinessctl check` returns `ready=true`.
