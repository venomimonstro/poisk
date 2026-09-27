# Sprint 28 — Readiness hardening addendum

**Status:** CODE/STATIC PASS — COMMERCIAL READY: NO

Additional hardening performed after the initial Sprint 28 audit:

- The final canonical `readiness.Gate` now independently verifies `artifact_ref + artifact_sha256` for generic commercial PASS evidence instead of trusting recorder history alone.
- PostgreSQL already enforces non-empty artifact refs, lowercase SHA-256, exact commit format and append-only evidence in `000063_commercial_readiness_evidence.sql`; no redundant migration was added.
- Readiness evidence `details` remains bounded to 16 KiB and now recursively rejects credential-bearing keys, including derived names such as `smtp_password`, `gateway_secret`, `access_token`, `dkim_private_key`, `api_key` and related suffix forms. Normal metrics such as `session_count` and `token_count` remain permitted.
- Unit regression coverage was added for nested/array secret-key detection.
- Owner Admin now exposes `/admin/readiness`, a read-only projection of the exact canonical `readiness.Gate`. It cannot set READY, record evidence or bypass a failing check.
- The Next Admin proxy exposes only explicit `GET readiness`; no new mutation route or generic backend tunnel was added.
- Admin readiness uses the same runtime contract as `readinessctl`: `READINESS_GIT_SHA`, `RELEASE_VERSION`, application Mail configuration and `MAIL_DKIM_SELECTOR`.
- `.github/workflows` remains absent.

The launch verdict is unchanged: **NOT READY** until real exact-release runtime evidence exists and the canonical gate returns `ready=true`.
