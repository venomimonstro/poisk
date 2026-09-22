# CURRENT SPRINT

**Sprint:** 25 — Mail Core
**Status:** IN_PROGRESS

## Goal
Создать безопасную first-party почту и внутреннюю систему сообщений на canonical consumer accounts без преждевременного SMTP/IMAP gateway.

## Depends On
- Sprint 00–21 — PASS (code/static gate where noted in reports)
- Sprint 22 — PASS code/static gate; runtime/browser/migration evidence remains external launch gate
- Sprint 23 — PASS code/static gate; runtime/recovery/capacity evidence remains external launch gate
- Sprint 24 — PASS code/static gate; runtime/browser/migration evidence remains external launch gate

## Allowed Work
- canonical mailbox and unique internal address tied to `consumer_users`
- Inbox / Sent / Drafts / Trash / Spam system folders
- messages, recipients, threads, read/star flags
- compose / save draft / send / reply / forward for internal recipients
- transactionally atomic internal delivery
- plain text plus sanitized HTML representation
- attachment metadata, SHA-256 content hash, owner/storage quotas
- bounded local attachment blob storage using opaque IDs; original filename never becomes a filesystem path
- attachment download with non-executable headers and tenant authorization
- PostgreSQL full-text search for mailbox search
- send / recipient / storage rate limits
- delete / restore / trash / retention semantics
- strict mailbox tenant isolation / IDOR / stored-XSS / path traversal tests
- consumer Mail UI using existing canonical account/session/CSRF layer
- Admin read-only Mail diagnostics necessary for abuse/support, without exposing message bodies by default

## Forbidden Work
- Internet SMTP/IMAP ingress or egress
- open relay or anonymous sending
- DKIM/SPF/DMARC implementation (Sprint 26)
- external email addresses as deliverable recipients
- using attachment filename as storage path
- automatic execution or inline active rendering of unsafe attachment types
- exposing absolute storage paths
- Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch
- GitHub Actions/CI
- changing Search/GEO organic ranking
- Sprint 26+ scope

## Definition of Done
- [ ] mailbox/address schema is migration-safe and one canonical mailbox belongs to one consumer
- [ ] system folders exist and tenant ownership is enforced server-side
- [ ] internal send is atomic: sender Sent + every recipient Inbox or none
- [ ] recipient enumeration is bounded and invalid recipients cannot create partial delivery
- [ ] drafts can be created/updated/sent without cross-tenant access
- [ ] reply and forward preserve thread/provenance without trusting client-owned mailbox IDs
- [ ] message body plain text is bounded; HTML is sanitized before storage/rendering
- [ ] attachment storage uses opaque IDs + SHA-256 + quotas; no user filename reaches filesystem path
- [ ] attachment download uses authorization + safe Content-Disposition/Content-Type/X-Content-Type-Options
- [ ] mailbox search uses PostgreSQL FTS with bounded query/limit
- [ ] send/recipient/storage abuse limits are enforced atomically
- [ ] trash/delete/restore semantics are explicit and bounded
- [ ] Mail API mutations require canonical consumer session + CSRF
- [ ] IDOR/tenant-isolation/stored-XSS/path-traversal/quota tests exist
- [ ] Mail UI supports Inbox/Sent/Drafts/Trash/Spam, compose, thread reading and search
- [ ] no SMTP/IMAP or Sprint 26 work is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 25 report created
