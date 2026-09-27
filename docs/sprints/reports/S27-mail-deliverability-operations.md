# Sprint 27 — Mail Deliverability Operations

**Status:** CODE/STATIC PASS · runtime evidence pending

## Implemented

- deterministic delivery failure classification: hard / transient / unknown;
- mailbox-scoped exact-recipient suppression using SHA-256 address identity, bounded hard-bounce threshold and expiry;
- suppression enforcement before enqueue and re-check inside the outbound lease transaction, so a READY/RETRY race cannot reach the MTA;
- idempotent MTA callbacks keep suppression and aggregate side effects replay-safe;
- bounded per-domain pressure window with server-side cooldown: 5m / 15m / 30m maximum for repeated transient failures;
- permanent recipient failures do not cool down an entire remote domain;
- outbound lease skips domains with active cooldown without client-controlled bypass;
- periodic MX/SPF/DMARC/DKIM readiness checks with immutable snapshots, bounded reason codes and drift fingerprint;
- no raw DNS TXT values, DKIM private keys, recipient lists, bounce bodies or DSNs are persisted in owner diagnostics;
- stale SUBMITTED deliveries older than 24h reconcile to DEAD/SUBMITTED_STALE and are never auto-resent;
- bounded retention for replay guard, gateway events, ordinary outbound audit events, DNS snapshots and inactive domain pressure;
- DEAD_RETRY authorization events are retained because they enforce the lifetime manual retry cap;
- explicit dead-letter retry is limited to retryable transport/transient codes, re-checks recipient suppression and allows at most three manual retries;
- dead-letter retry is exposed to Admin only through session-bound, CSRF-protected preview/apply; delivery state/error/version is revalidated before apply;
- Admin dead-letter list intentionally excludes recipient address, message body and full recipient lists;
- Internet Mail operations UI exposes queue aggregates, suppression count, cooling domains, DNS readiness/drift and privacy-safe dead letters;
- mail-gateway-worker owns DNS monitoring and deliverability maintenance; no new infrastructure/service family was introduced;
- fixed Admin VIEWER role validation inconsistency discovered during the Sprint 27 security audit.

## Migrations

- `000060_mail_deliverability_ops.sql` — suppression, domain pressure, base DNS readiness snapshots, outbound event actions.
- `000061_mail_dns_readiness_snapshots.sql` — upgrade-safe drift/fingerprint extension and retention-compatible immutability.
- `000062_admin_mail_dead_retry_previews.sql` — protected Admin preview/apply action for dead-letter retry.

Migration 061 was explicitly corrected to ALTER the table created by 060 rather than attempting to recreate it.

## Regression coverage added

- bounded cooldown thresholds and cap;
- remote-domain normalization;
- active domain cooldown blocks lease and expiry restores eligibility;
- active recipient suppression is re-checked at lease boundary;
- stale SUBMITTED reconciles to DEAD and does not auto-resend;
- DNS snapshots only mark drift when readiness changes;
- dead-letter retry whitelist rejects permanent/suppression/configuration failures.

Integration tests require `TEST_DATABASE_URL` and the migrated PostgreSQL schema.

## Security / privacy properties

- PostgreSQL remains the source of truth;
- no public SMTP listener was added to the Go application;
- no Redis/Kafka/RabbitMQ/new queueing infrastructure;
- no global cross-tenant recipient blacklist;
- no raw DSN/body storage in deliverability operations;
- no recipient address exposure in Owner Admin dead-letter diagnostics;
- all manual mutations are server-authorized and audited;
- `.github/workflows` remains absent; no GitHub Actions/CI was added.

## Runtime gate still required

This report does **not** claim production/runtime PASS. Before commercial release the deployment environment must still execute:

1. full Go build/unit/integration suite;
2. fresh migration 001→062;
3. upgrade migration from a previous production-like database;
4. outbound MTA submission/callback replay tests;
5. real DNS readiness/drift test for the configured mail domain;
6. suppression/cooldown/dead-retry browser integration;
7. recovery and Capacity evidence required by the commercial gate.

## Result

Sprint 27 implementation is complete at code/static level. Runtime evidence is intentionally deferred to the final Security/Commercial Readiness gate rather than being reported as a false PASS.
