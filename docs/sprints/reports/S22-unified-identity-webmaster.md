# Sprint 22 — Unified Identity + Webmaster Cabinet

**Status:** PASS — code/static gate. Runtime/browser/migration evidence remains mandatory before commercial launch and is explicitly not claimed here.

## Implemented

- Canonical `consumer_users` identity independent from Admin identities.
- Backfill/link from existing Webmaster users without changing historical Webmaster IDs.
- Argon2id for new consumer passwords with PBKDF2 compatibility and login-time rehash.
- Consumer/legacy Webmaster password hashes synchronized transactionally after rehash.
- Hashed session and CSRF tokens; raw session token is only delivered as an HttpOnly cookie.
- SameSite=Strict consumer session cookie, environment-aware Secure flag.
- Server-side session expiry/revocation, active-session cap, login failure lockout and security events.
- Legacy Webmaster Bearer auth retained for integrations; browser Webmaster uses consumer cookie + CSRF.
- Browser facade resolves consumer -> Webmaster profile server-side; client cannot choose another user identity.
- Existing `OwnedSite` checks remain the tenant-isolation source of truth for site operations.
- `/webmaster` UI: login/register, site list/add, DNS/HTML/META verification, metrics, sitemap submit, URL submit/reindex/delete, URL diagnostics, plan/usage and session management.
- URL diagnostics expose HTTP/crawl/index status, latest crawler error, canonical URL, robots noindex/nofollow.
- Webmaster Pro usage endpoint resolves billing account server-side and exposes current plan/limits/usage without accepting client account IDs.
- Billing API is wired into API runtime.
- Public search navigation links Search / Maps / Webmaster / Data.
- Integration suite covers consumer session isolation, active-session cap, consumer-to-Webmaster mapping, password synchronization and legacy insert compatibility.
- Migration loader rejects duplicate numeric migration versions; historical duplicate 000018/000019 variants were removed while idempotent repair migration 000024 preserves convergence.
- Existing Admin identity/session/2FA remains a separate security domain.

## Critical defects found and fixed during audit

1. Duplicate `Manticore.Client.CountDocuments` implementations would cause a compile failure; the stale duplicate implementation/test were removed.
2. Capacity document-count consistency had dead/unwired logic; real PostgreSQL/Manticore counts are collected.
3. Capacity benchmark could complete without server CPU/RAM/disk evidence; immutable completion fails closed without server measurements.
4. Existing Manticore installations could miss `fetched_at`; schema upgrade path was added.
5. New route registrars incorrectly accepted `*guard.Middleware` while runtime supplied a value; signatures were corrected.
6. Legacy Webmaster inserts conflicted with `consumer_user_id NOT NULL`; compatibility migration links an existing consumer or creates one.
7. Duplicate compatibility migration was removed and migration logic consolidated.
8. Billing routes existed but were not mounted in `runAPI`; they are registered.
9. Consumer password rehash could diverge from legacy Webmaster auth; both hashes update transactionally.
10. Historical duplicate migration versions made fresh-install ordering nondeterministic; duplicate files were removed and the migration loader now fails fast on any future duplicate version.

## Verification status

- Static/code review gate: PASS.
- Unit/integration test code: added/expanded.
- Runtime execution in the working container: BLOCKED because the container cannot resolve `github.com`; a fresh clone/build cannot be executed here.
- Therefore no claim is made that Go build, frontend build, migrations, browser E2E or production-like upgrade tests passed at runtime.

## Mandatory external launch gate

Before commercial launch, execute Go/frontend builds, integration tests, previous-database migration upgrade, browser register/login/cookie/CSRF/logout/session-revoke flows, controlled DNS/HTTP ownership verification fixtures, analytics with realistic indexed data, and the global Capacity/Security commercial gate.
