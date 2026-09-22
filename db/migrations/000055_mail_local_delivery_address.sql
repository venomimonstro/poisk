ALTER TABLE mail_recipients ADD COLUMN delivery_address TEXT;

UPDATE mail_recipients r
SET delivery_address=mb.address
FROM mailboxes mb
WHERE mb.mailbox_id=r.recipient_mailbox_id AND r.delivery_address IS NULL;

ALTER TABLE mail_recipients ALTER COLUMN delivery_address SET NOT NULL;
ALTER TABLE mail_recipients ADD CONSTRAINT mail_recipients_delivery_address_length CHECK (length(delivery_address) BETWEEN 3 AND 320);
CREATE INDEX idx_mail_recipients_delivery_address ON mail_recipients(lower(delivery_address),message_id);
