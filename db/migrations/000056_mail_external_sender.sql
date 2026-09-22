ALTER TABLE mail_messages ALTER COLUMN sender_mailbox_id DROP NOT NULL;
ALTER TABLE mail_messages ADD COLUMN IF NOT EXISTS external_sender_address TEXT;

ALTER TABLE mail_messages DROP CONSTRAINT IF EXISTS mail_messages_provenance_check;
ALTER TABLE mail_messages
    ADD CONSTRAINT mail_messages_provenance_check
    CHECK (provenance IN ('COMPOSE','REPLY','FORWARD','INTERNET_INBOUND'));

ALTER TABLE mail_messages DROP CONSTRAINT IF EXISTS mail_messages_sender_identity_check;
ALTER TABLE mail_messages
    ADD CONSTRAINT mail_messages_sender_identity_check
    CHECK (
        (sender_mailbox_id IS NOT NULL AND external_sender_address IS NULL AND provenance <> 'INTERNET_INBOUND') OR
        (sender_mailbox_id IS NULL AND external_sender_address IS NOT NULL AND provenance = 'INTERNET_INBOUND')
    );

ALTER TABLE mail_messages DROP CONSTRAINT IF EXISTS mail_messages_external_sender_check;
ALTER TABLE mail_messages
    ADD CONSTRAINT mail_messages_external_sender_check
    CHECK (
        external_sender_address IS NULL OR
        (length(external_sender_address) BETWEEN 3 AND 320 AND position('@' in external_sender_address) > 1)
    );

CREATE INDEX IF NOT EXISTS idx_mail_messages_external_sender
    ON mail_messages(lower(external_sender_address),created_at DESC,message_id DESC)
    WHERE external_sender_address IS NOT NULL;
