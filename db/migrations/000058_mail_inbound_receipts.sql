CREATE TABLE mail_inbound_receipts (
    event_id TEXT PRIMARY KEY CHECK (length(event_id) BETWEEN 16 AND 160),
    body_sha256 CHAR(64) NOT NULL CHECK (body_sha256 ~ '^[0-9a-f]{64}$'),
    envelope_recipient TEXT NOT NULL CHECK (length(envelope_recipient) BETWEEN 3 AND 320),
    mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    message_id BIGINT REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'PROCESSING' CHECK (status IN ('PROCESSING','ACCEPTED','REJECTED')),
    rejection_code TEXT CHECK (rejection_code IS NULL OR length(rejection_code) <= 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK ((status='ACCEPTED' AND message_id IS NOT NULL AND completed_at IS NOT NULL) OR status<>'ACCEPTED'),
    CHECK ((status='REJECTED' AND rejection_code IS NOT NULL AND completed_at IS NOT NULL) OR status<>'REJECTED')
);
CREATE INDEX idx_mail_inbound_receipts_mailbox ON mail_inbound_receipts(mailbox_id,created_at DESC,event_id);
CREATE INDEX idx_mail_inbound_receipts_message ON mail_inbound_receipts(message_id) WHERE message_id IS NOT NULL;
