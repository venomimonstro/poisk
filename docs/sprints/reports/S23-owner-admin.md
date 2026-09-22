# Sprint 23 — Owner Admin Console

**Status:** PASS — code/static gate. Runtime/build/browser/integration/capacity/recovery evidence remains mandatory before commercial launch.

## Implemented

- Modular Owner Admin console with shared navigation: Overview / System / Support / Data Hub / Organizations.
- RBAC supports `VIEWER`, `ANALYST`, `OPERATOR`, `SUPERADMIN` while retaining legacy `SUPPORT`; SUPERADMIN inherits operator permissions.
- Read-only subsystem diagnostics for crawler queue/history, index/outbox, Demand/Query Gaps, Webmaster, billing, Data Hub and Capacity.
- Consumer users, sessions and security events are exposed without password hashes, session hashes or raw auth tokens.
- Webmaster site/support diagnostics and billing invoice diagnostics are available without direct PostgreSQL access.
- Query Gap suppress/reopen uses CSRF + session-bound one-time preview/apply + immutable audit.
- Domain policy mutations retain CSRF + preview/apply + audit.
- Data Hub SUPPRESS / REOPEN / ROLLBACK uses version-aware preview/apply; stale previews fail closed.
- Organization import batches and merge-review queue are visible to read-only roles.
- Organization MERGE / CREATE_NEW / REJECT decisions use CSRF + session-bound preview/apply, re-check OPEN review state under lock, update the canonical import plan and write both domain and global audit events.
- Maps diagnostics expose active/previous PMTiles versions, checksums, size and zoom bounds.
- Address diagnostics expose active/inactive counts, orphan/ignored counters and latest GAR/FIAS import revision/status.
- Search Quality runs are now persisted in immutable `quality_runs`; Admin exposes the latest real NDCG/MRR/Recall/zero-result/duplicate gate result.
- Answer Engine observability uses privacy-safe hourly aggregates only; no query text, IP, User-Agent or per-user history is stored.
- Recovery readiness is fail-closed: immutable BACKUP and RESTORE drill evidence is required at the current database schema along with an active release.
- `recoveryctl record` records evidence metadata/checksum/size/duration but never stores backup contents or secrets.
- Release state remains owned by the existing releasectl/preflight/activate/rollback subsystem rather than duplicated in Admin.
- Integration regression added for organization review preview session binding, single use and dual audit.
- Shared Admin UI navigation added.

## Critical findings fixed during Sprint 23

1. Historical migration handling was strengthened earlier with duplicate-version fail-fast.
2. A new duplicate numeric migration was caught during this sprint: Data Hub already occupied `000042`; recovery evidence was incorrectly created as another `000042`. Recovery was moved to unique migration `000046` before sprint closure.
3. Address diagnostics previously swallowed database errors when loading the latest import batch; it now fails loudly instead of presenting missing data as healthy state.
4. HTTP and service RBAC were aligned so VIEWER read access no longer receives a hidden service-layer 403.
5. Search Quality previously existed only as stdout output; persistent immutable gate history was added.
6. Answer Engine previously had no owner-facing runtime telemetry; privacy-safe bounded aggregates were added without retaining query text.
7. Organization moderation was not safe to expose directly from `orgctl`; web Admin now uses a dedicated preview/apply transaction and immutable audit path.

## Security properties

- Admin authentication remains isolated from consumer identity.
- Admin mutations require authenticated role + CSRF and, for critical state changes, preview/apply.
- Preview tokens are hashed, session-bound, expiring and single-use.
- Read endpoints do not expose password hashes, session/auth token hashes, TOTP secrets, payment payload hashes or raw payment payloads.
- Quality/Answer telemetry does not introduce user/query histories.
- No paid/billing signal is introduced into organic Search or Maps ranking.
- No GitHub Actions / `.github/workflows` were added.

## Runtime evidence still required

- Go build/unit tests in a runnable dependency environment.
- PostgreSQL migration upgrade from a production-like pre-Sprint-22/23 snapshot.
- Admin browser integration across VIEWER/ANALYST/OPERATOR/SUPERADMIN.
- Organization review and Data Hub integration tests against migrated PostgreSQL.
- A real Search Quality run to populate `quality_runs`.
- Real BACKUP and RESTORE drills recorded at the final schema version.
- Capacity benchmark evidence on the intended commercial host.

## Commercial readiness

**NOT READY.** Sprint 23 removes major owner-operability gaps but does not replace the outstanding runtime/capacity/recovery evidence and future product/security gates. Sprint 24 (Maps + Reviews), Mail scope and Sprint 27 security/commercial readiness remain required by the current roadmap.
