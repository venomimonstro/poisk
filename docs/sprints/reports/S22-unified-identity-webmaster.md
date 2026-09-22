# Sprint 22 — Unified Identity + Webmaster Cabinet

**Status:** IN_PROGRESS — code/static implementation substantially complete; runtime/integration gate pending.

## Implemented

- Canonical `consumer_users` identity independent from Admin identities.
- Backfill/link from existing Webmaster users without changing historical Webmaster IDs.
- Argon2id for new consumer passwords with PBKDF2 compatibility and login-time rehash.
- Consumer/legacy Webmaster password hashes synchronized after rehash.
- Hashed session and CSRF tokens; raw session token is only delivered as an HttpOnly cookie.
- SameSite=Strict consumer session cookie, environment-aware Secure flag.
- Server-side session expiry/revocation, active-session cap, login failure lockout and security events.
- Legacy Webmaster Bearer auth retained for integrations; browser Webmaster uses consumer cookie + CSRF.
- Browser facade resolves consumer -> Webmaster profile server-side; client cannot choose another user identity.
- Existing `OwnedSite` checks remain the tenant-isolation source of truth for site operations.
- `/webmaster` UI: login/register, site list/add, DNS/HTML/META verification, metrics, sitemap submit, URL submit/reindex/delete, URL status.
- URL diagnostics expose HTTP/crawl/index status, latest crawler error, canonical URL, robots noindex/nofollow.
- Webmaster Pro usage endpoint resolves billing account server-side and exposes current plan/limits/usage without accepting client account IDs.
- Billing API was wired into API runtime; previously implemented routes are no longer dead code.
- Public search navigation links Search / Maps / Webmaster / Data.
- Regression coverage for Argon2 compatibility and CSRF mismatch.
- Existing Admin identity/session/2FA remains a separate security domain.

## Critical defects found and fixed during audit

1. Duplicate `Manticore.Client.CountDocuments` implementations would cause a compile failure; the stale duplicate implementation/test were removed.
2. Capacity document-count consistency had dead/unwired logic; real PostgreSQL/Manticore counts are now collected.
3. Capacity benchmark could complete without server CPU/RAM/disk evidence; immutable completion now fails closed without server measurements.
4. Existing Manticore installations could miss `fetched_at`; schema upgrade path was added.
5. New route registrars incorrectly accepted `*guard.Middleware` while runtime supplied a value; signatures were corrected.
6. Legacy Webmaster inserts conflicted with `consumer_user_id NOT NULL`; compatibility migration now links an existing consumer or creates one.
7. Duplicate compatibility migration was removed and migration logic consolidated.
8. Billing routes existed but were not mounted in `runAPI`; they are now registered.
9. Consumer password rehash could diverge from legacy Webmaster auth; both hashes now update transactionally.
10. Owner dashboard now counts temporary login lockouts, not only persistent `LOCKED` status.

## Still required before Sprint 22 PASS

- Execute Go build/test and frontend type/build against a runnable dependency environment.
- Apply migrations from a previous production-like database snapshot and verify upgrade/rollback procedure.
- Browser integration tests for register/login/cookie/CSRF/logout/session revoke.
- Database integration tests for consumer-to-Webmaster mapping and explicit cross-tenant denial.
- Surface account session management and Webmaster plan/usage in the UI (backend endpoints are ready).
- Validate full verification flows against controlled DNS/HTTP fixtures.
- Confirm analytics/usage with realistic indexed data.

## Commercial readiness

Sprint 22 does **not** make the product commercially ready by itself. Commercial launch remains blocked by runtime migration/build evidence, Capacity Gate evidence, Maps/Reviews completion, mail scope completion where required, and the final security/commercial readiness gate.
