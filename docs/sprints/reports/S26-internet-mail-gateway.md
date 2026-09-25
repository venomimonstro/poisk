# Sprint 26 — Internet Mail Gateway

**Result:** PASS — code/static gate. Runtime/build/live-MTA/DNS/browser/integration evidence remains an external launch gate.

## Delivered

- Sprint 25 Mail Core remains canonical for users, mailboxes, aliases, threads, messages, recipients and attachments.
- Internet recipients are represented separately from local mailbox recipients; browser clients do not choose an SMTP relay or submit relay credentials.
- Outbound Internet delivery uses a durable PostgreSQL queue with leases, retries, dead-letter state, idempotency keys and audit events.
- `mail-gateway-worker` is an explicit runtime mode and is disabled unless Internet Mail configuration is enabled and DNS readiness succeeds.
- The application submits a bounded canonical `MTAEnvelope` to one configured private MTA bridge endpoint. Requests are timestamped and HMAC signed.
- Inbound gateway calls are authenticated, timestamp bounded and replay protected before recipient validation, MIME persistence or delivery-state updates.
- Local recipient validation accepts only active first-party mailbox/alias identities; there is no wildcard local catch-all path in the application contract.
- Inbound MIME parsing is bounded before persistence. HTML remains inert application data and attachments remain opaque blobs with size/hash metadata.
- Delivery callbacks reconcile durable outbound state to delivered/bounced terminal states without relying on browser state.
- Mail UI exposes Internet delivery state through the existing first-party API rather than direct MTA access.
- Admin mail-gateway health is aggregate-only; routine health does not expose message bodies or recipient lists.

## Production boundary

- Added explicit Internet Mail environment contract to `.env.example`; Internet delivery remains OFF by default.
- Added `deploy/mail/README.md` describing the application/MTA trust boundary, signed private bridge, recipient validation and rollback order.
- Added `deploy/mail/postfix-main.cf.example` with default-deny relay policy, active-recipient map boundary, bounded SMTP resource limits and TLS/DKIM secret mounts.
- Added `deploy/mail/validate-boundary.sh` static guard for required relay restrictions and obvious wildcard/universal-trust mistakes.
- DKIM private key material is intentionally outside the Go application, PostgreSQL, repository and browser. Only selector/expected public TXT are application configuration.
- Added `mailctl dns-check` to perform an operator preflight for MX, SPF, DMARC and DKIM before Internet Mail is enabled.

## Security/test coverage

- HMAC authentication, clock-window rejection and replay protection.
- Recipient isolation and unknown/inactive recipient rejection.
- MIME and attachment bounds plus inert content handling.
- Durable outbound idempotency/retry/delivery-event behavior.
- Signed fixed MTA submission contract; weak secret and invalid relay URL rejection; deterministic 4xx/429/5xx classification.
- Deterministic DNS readiness tests covering ready, missing/mismatched records and resolver failure.
- Static Postfix boundary validator requires `reject_unauth_destination`, explicit recipient maps and bounded resource settings while rejecting universal trusted networks/wildcard maps.

## Operational enablement

1. Deploy schema/application with `MAIL_INTERNET_ENABLED=false`.
2. Configure the private MTA bridge and default-deny Postfix boundary.
3. Mount TLS/DKIM secrets on the MTA host; publish MX/SPF/DMARC/DKIM.
4. Configure a random shared gateway secret of at least 32 bytes on both sides.
5. Run `app mailctl dns-check` and `sh deploy/mail/validate-boundary.sh` in the deployment environment.
6. Prove unknown-local-recipient rejection and external-to-external relay rejection against the real MTA.
7. Enable the Compose `internet-mail` profile and start `mail-gateway-worker`.
8. Observe aggregate gateway health and durable queue/dead-letter counts.

Rollback is bounded: disable `MAIL_INTERNET_ENABLED` and stop `mail-gateway-worker`. Canonical Mail Core state remains intact.

## Runtime gate still required

No live runtime claim is made from this session. Before production enablement, the deployment environment must still provide evidence for full Go build/tests, migrations from the previous production schema, live Postfix/MTA bridge integration, public DNS propagation, TLS/DKIM signing, open-relay negative tests, inbound/outbound end-to-end delivery, browser flows and rollback/recovery.

## Scope boundary preserved

Sprint 26 did **not** add a custom public SMTP server to the Go application, IMAP/POP3, wildcard forwarding, browser-visible SMTP/DKIM secrets, unauthenticated inbound paths, Redis/Kafka/another broker, search-ranking changes, or GitHub Actions/CI.
