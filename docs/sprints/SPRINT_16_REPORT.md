# Sprint 16 — Hardening Report

**Status:** PASS (code/static gate)

## Delivered

### Admin / RBAC / 2FA
- Admin authentication is isolated from Webmaster/public auth.
- Admin API fails closed when `ADMIN_SECRET_KEY_B64` is unavailable.
- Argon2id password hashing, opaque hashed session tokens, bounded/revocable sessions.
- TOTP 2FA with AES-256-GCM encrypted secret and one-time hashed recovery codes.
- Roles: SUPERADMIN, OPERATOR, ANALYST, SUPPORT; authorization is enforced server-side.
- HttpOnly session cookie, SameSite=Strict, Secure outside development.
- CSRF token lifecycle including authenticated token rotation.
- Login lockout and security-event recording.

### Safe Admin mutations
- Domain status/policy mutations require server-side RBAC + CSRF.
- Mutation flow is PREVIEW → five-minute one-time token → APPLY.
- Preview tokens are tied to admin + session and stored only as hashes.
- Mutation + preview consumption + audit record are atomic.
- `/admin` UI exposes bounded operational data and domain management without SQL/secret access.

### Operational visibility
- Admin status exposes bounded crawler, outbox, import, runtime and resource-pressure metrics.
- No query text, credentials, SQL, raw private payloads or secrets are returned.
- nginx baseline response headers: nosniff, DENY framing, strict referrer policy and restrictive permissions policy.

### Disk / memory protection
- Dedicated resource monitor records filesystem and cgroup-memory pressure.
- Failed sampling becomes CRITICAL instead of silently allowing heavy work.
- Stale pressure records (>60 seconds) fail closed.
- CRITICAL/stale pressure prevents new crawler leases, index outbox leases, organization APPLY leases and Webmaster sitemap leases.
- Existing leased work may finish and Search/API remains available.
- Integration test code covers CRITICAL and stale lease rejection.

### Backup / recovery
- PostgreSQL custom-format backup with archive listing, manifest and SHA-256 checksums.
- Secrets are deliberately excluded.
- Non-destructive backup verification command added.
- Restore requires exact database confirmation and verified backup metadata.
- Non-empty target replacement requires exact `RESTORE_ALLOW_NONEMPTY=yes`.
- Fixed a destructive shell-expansion bug so `--clean --if-exists` is used only for an explicitly approved non-empty restore.
- Web, organization and address Manticore indexes all have PostgreSQL rebuild commands and are treated as disposable.

### Release / rollback
- Release registry records version, build, DB schema, map version, all Manticore schema versions, config hash and immutable backend/frontend image refs.
- Manifest fields are shell-safe and covered by injection-guard tests.
- Candidate backend performs compatibility preflight.
- Apply sequence: candidate env → candidate preflight → candidate containers → health check → registry activation.
- Failed candidate health does not change ACTIVE registry state and containers are returned to the previous image pair when available.
- Rollback sequence similarly health-checks the previous release before switching registry pointers.
- Rollback never rewinds canonical PostgreSQL data or performs destructive migration shortcuts.

### Tests / documentation
- Admin crypto unit tests: password hash, AES-GCM tamper rejection, recovery codes, TOTP window.
- Admin DB integration test code: session revocation, preview session binding and single use.
- Resource-pressure integration test code for crawl and index queues.
- Existing organization/Webmaster integration fixtures updated for pressure contract.
- Release manifest injection tests.
- Production Hardening Runbook documents bootstrap, backup, verification, restore, index rebuild, release, rollback and restore-test procedure.
- `.github/workflows` does not exist; no GitHub Actions/CI was added.

## Gate limitation

The execution environment used for this development session cannot resolve `github.com` from the container, so dependency download / clean clone / `go test ./...` / frontend install-build and a real Docker migration/restore exercise cannot be honestly executed here. This sprint is therefore recorded as a **code/static gate**, not as an executed production acceptance test.

Before production activation, the runbook requires an environment with dependency/network access to execute unit/integration tests, migrations from a previous database, Docker health checks and an isolated backup restore test.

## Known intentional constraints
- No Kubernetes, Redis, Kafka/RabbitMQ or external monitoring SaaS was introduced.
- No Sprint 17 Growth features were included in this sprint.
- No billing or paid-ranking functionality was introduced.
