ALTER TABLE mail_messages ALTER COLUMN sender_mailbox_id DROP NOT NULL;
ALTER TABLE mail_messages ADD COLUMN sender_external_address TEXT;
ALTER TABLE mail_messages ADD COLUMN internet_message_id TEXT;
ALTER TABLE mail_messages ADD COLUMN reply_to_address TEXT;
ALTER TABLE mail_messages ADD COLUMN received_at TIMESTAMPTZ;

ALTER TABLE mail_messages DROP CONSTRAINT IF EXISTS mail_messages_provenance_check;
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_provenance_check CHECK (provenance IN ('COMPOSE','REPLY','FORWARD','INBOUND'));
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_sender_xor CHECK ((sender_mailbox_id IS NOT NULL) <> (sender_external_address IS NOT NULL));
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_sender_external_length CHECK (sender_external_address IS NULL OR length(sender_external_address) BETWEEN 3 AND 320);
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_internet_id_length CHECK (internet_message_id IS NULL OR length(internet_message_id) <= 998);
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_reply_to_length CHECK (reply_to_address IS NULL OR length(reply_to_address) <= 320);
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_inbound_received CHECK ((provenance='INBOUND' AND received_at IS NOT NULL) OR provenance<>'INBOUND');
CREATE INDEX idx_mail_messages_internet_id ON mail_messages(internet_message_id) WHERE internet_message_id IS NOT NULL;
CREATE INDEX idx_mail_messages_received ON mail_messages(received_at DESC,message_id DESC) WHERE provenance='INBOUND';
