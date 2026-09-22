# Sprint 25 — Mail Core

**Result:** PASS — code/static gate. Runtime/build/browser/integration evidence remains an external launch gate.

## Implemented
- Canonical one-to-one mailbox on `consumer_users` with opaque non-enumerable `@internal.poisk` address.
- Server-owned Inbox / Sent / Drafts / Trash / Spam folders with DB tenant ownership guard.
- Internal messages, recipients, threads, read/star flags, reply/forward provenance and PostgreSQL FTS.
- Draft create/update/reopen/send flow. Saved drafts are edited in place and retain `parent_message_id` / thread provenance.
- Atomic send transaction: sender Sent copy plus all recipient Inbox deliveries or none.
- Send now locks all recipient/mailbox rows and fails closed if any recipient becomes disabled before delivery.
- Bounded recipient enumeration and internal-only resolution. External addresses are not deliverable in this sprint.
- BCC privacy: ordinary recipients do not see BCC recipients; sender and the BCC recipient retain appropriate visibility.
- Plain text canonical body and escaped/sanitized HTML representation.
- Attachments up to bounded size with opaque UUID storage keys, SHA-256, mailbox quota, upload-rate accounting, original filename metadata only.
- Tenant-safe attachment metadata/download; non-executable download headers (`attachment`, `application/octet-stream`, `nosniff`, CSP sandbox).
- Draft attachment detach endpoint and UI; attachment list/download in message view.
- Trash/restore/spam semantics and 30-day trash retention.
- Blob GC and action-bucket retention run from the existing resource-monitor maintenance runtime every six hours.
- Mail UI for Inbox/Sent/Drafts/Trash/Spam, compose, saved-draft editing, reply, forward, search, attachments and mailbox quota display.
- Canonical consumer HttpOnly session + CSRF on all Mail mutations.
- Privacy-safe Owner Admin Mail diagnostics: mailbox/message/item counts, attachment bytes, GC backlog and 24h sends; no subjects, bodies, recipient addresses or filenames.

## Security / regression coverage
- cross-tenant item access
- send all-or-none
- invalid recipient cannot create partial delivery
- recipient disabled between draft and send cancels all delivery
- self-send creates separate Sent and Inbox copies without duplicate recipient rows
- draft edit tenant isolation and reply-parent/thread preservation
- BCC visibility boundaries
- attachment filename traversal rejection
- attachment cross-tenant resolve/detach rejection
- opaque storage path and blob GC
- trash retention waits for all mailbox copies before physical message/blob deletion
- stored-XSS escaping and Content-Type header-injection hardening

## Important fixes found during Sprint 25 audit
1. **Partial delivery bug:** inactive recipient rows were previously filtered out during send; other recipients could still receive the message. Fixed by locking every resolved recipient/mailbox row and rejecting the entire transaction if any mailbox is not ACTIVE.
2. **Saved draft UX/data bug:** opening compose could inherit the currently selected message and saved DRAFT items could not be safely continued. Added a tenant-safe DraftEdit read model and explicit compose/reply/forward/edit modes.
3. **Attachment usability gap:** recipients could download only if an attachment ID was already known. Added tenant-safe item attachment listing, safe message links and draft detach.
4. **Owner observability gap:** Mail had no Admin health snapshot. Added aggregate diagnostics only, deliberately excluding private message content.

## Runtime gate
A real clone/build/test probe was attempted from the execution container. It failed before compilation because the environment could not resolve `github.com`:

`fatal: unable to access 'https://github.com/venomimonstro/poisk.git/': Could not resolve host: github.com`

Therefore this report does **not** claim runtime tests or migrations passed. Required external launch evidence remains:
- `go test` including integration tests with `TEST_DATABASE_URL`
- fresh migration 001→latest and upgrade from production-like previous schema
- Next.js build/browser flow
- resource-monitor retention/GC run against mounted blob volume
- capacity/recovery gates from earlier sprints

## Scope boundary
No SMTP/IMAP ingress/egress, open relay, DKIM, SPF or DMARC was added in Sprint 25. No GitHub Actions/CI was added.
