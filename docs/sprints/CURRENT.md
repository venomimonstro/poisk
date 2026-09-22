# CURRENT SPRINT

**Sprint:** 26 — Internet Mail Gateway
**Status:** IN_PROGRESS

## Goal
Расширить first-party Mail Core безопасной отправкой и приёмом интернет-почты через отдельную проверенную MTA-границу, не превращая Go-приложение в самописный публичный SMTP relay.

## Depends On
- Sprint 00–24 — PASS (code/static gate where noted in reports)
- Sprint 25 — PASS code/static gate; runtime/browser/migration evidence remains external launch gate

## Architecture Boundary
- PostgreSQL + Go Mail Core остаются source of truth для mailbox/message/thread/items.
- Публичный SMTP принимает/передаёт отдельный MTA service (Postfix-compatible boundary); application API не слушает публичный SMTP порт.
- MTA не получает consumer session/password hashes и не пишет напрямую в canonical Mail tables.
- Inbound передаётся в приложение только через authenticated internal gateway contract.
- Outbound попадает в MTA только из durable server-side queue; браузер никогда не управляет SMTP host/credentials.

## Allowed Work
- external recipient representation alongside existing internal recipients
- durable outbound queue with idempotency, lease/retry/dead states and bounded exponential backoff
- trusted-MTA inbound ingestion contract with HMAC/rotatable shared secret or equivalent internal authentication
- bounded RFC 5322/MIME parsing after MTA acceptance
- Internet From/To/Cc/Reply-To metadata and provenance without exposing BCC
- inbound/outbound attachment reuse through existing opaque blob/quota subsystem
- Postfix-compatible deployment boundary and non-open-relay configuration
- DKIM signing at MTA boundary; private key only as mounted secret/file, never PostgreSQL/browser
- SPF and DMARC DNS configuration/check tooling/documentation
- message size/recipient/rate/domain safety limits
- bounce/delivery-status persistence without leaking provider internals to other tenants
- external addresses in Mail UI compose/read views
- Admin aggregate gateway health: queue/retry/dead/bounce/inbound counts, no bodies by default
- integration/security tests for relay prevention, HMAC replay, MIME limits, external-recipient isolation and idempotent delivery

## Forbidden Work
- custom internet-facing SMTP protocol implementation in the application
- open relay / unauthenticated arbitrary outbound sending
- storing SMTP/DKIM secrets in PostgreSQL, source code or browser-visible config
- accepting inbound gateway calls without authentication/replay protection
- automatic inline execution/rendering of active MIME/HTML/attachments
- IMAP/POP3 server implementation in this sprint
- wildcard external forwarding rules
- Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch
- GitHub Actions/CI
- changing Search/GEO organic ranking
- Sprint 27+ scope

## Definition of Done
- [ ] internal-only delivery from Sprint 25 remains backwards-compatible
- [ ] external recipients persist separately and cannot be mistaken for local mailbox identities
- [ ] outbound external delivery is durable, idempotent and all queue state transitions are auditable
- [ ] queue leasing is crash-safe with retry/dead-letter semantics
- [ ] browser cannot select relay host/credentials or bypass per-account limits
- [ ] inbound gateway requires authenticated, replay-protected MTA request
- [ ] inbound MIME size/part/header/address limits are enforced before canonical persistence
- [ ] inbound message is delivered only to an existing ACTIVE local mailbox/alias
- [ ] external HTML is sanitized or rendered as inert text; unsafe active content is never executed
- [ ] attachments reuse opaque storage IDs, SHA-256 and quota enforcement
- [ ] no open relay; MTA relay rules are explicit and default-deny
- [ ] DKIM secret stays outside DB/source/browser and signing boundary is documented/testable
- [ ] SPF/DMARC required records can be checked deterministically
- [ ] bounces and final delivery failures become bounded canonical events/statuses
- [ ] Mail UI can compose to external addresses and distinguish delivery status
- [ ] Admin can see aggregate gateway health without message bodies/recipient lists
- [ ] SMTP/IMAP secrets are not logged
- [ ] no GitHub Actions/CI added
- [ ] Sprint 26 report created
