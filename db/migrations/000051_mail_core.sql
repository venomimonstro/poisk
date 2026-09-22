CREATE TABLE mailboxes (
    mailbox_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    consumer_user_id BIGINT NOT NULL UNIQUE REFERENCES consumer_users(user_id) ON DELETE RESTRICT,
    address TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    storage_quota_bytes BIGINT NOT NULL DEFAULT 104857600 CHECK (storage_quota_bytes BETWEEN 1048576 AND 10737418240),
    storage_used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (storage_used_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(address) BETWEEN 3 AND 320)
);
CREATE UNIQUE INDEX uq_mailboxes_address_ci ON mailboxes(lower(address));

CREATE TABLE mail_folders (
    folder_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    kind TEXT NOT NULL CHECK (kind IN ('INBOX','SENT','DRAFTS','TRASH','SPAM')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(mailbox_id,kind)
);

CREATE OR REPLACE FUNCTION mail_create_system_folders() RETURNS trigger AS $$
BEGIN
    INSERT INTO mail_folders(mailbox_id,kind)
    VALUES(NEW.mailbox_id,'INBOX'),(NEW.mailbox_id,'SENT'),(NEW.mailbox_id,'DRAFTS'),(NEW.mailbox_id,'TRASH'),(NEW.mailbox_id,'SPAM')
    ON CONFLICT(mailbox_id,kind) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_mailbox_system_folders AFTER INSERT ON mailboxes FOR EACH ROW EXECUTE FUNCTION mail_create_system_folders();

CREATE TABLE mail_threads (
    thread_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subject TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 998),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE mail_messages (
    message_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    thread_id BIGINT NOT NULL REFERENCES mail_threads(thread_id) ON DELETE RESTRICT,
    sender_mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    parent_message_id BIGINT REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    provenance TEXT NOT NULL DEFAULT 'COMPOSE' CHECK (provenance IN ('COMPOSE','REPLY','FORWARD')),
    subject TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 998),
    body_text TEXT NOT NULL CHECK (length(body_text) BETWEEN 1 AND 200000),
    body_html TEXT CHECK (body_html IS NULL OR length(body_html) <= 400000),
    state TEXT NOT NULL DEFAULT 'SENT' CHECK (state IN ('DRAFT','SENT')),
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple',coalesce(subject,'')),'A') ||
        setweight(to_tsvector('simple',coalesce(body_text,'')),'B')
    ) STORED,
    CHECK ((state='SENT' AND sent_at IS NOT NULL) OR state='DRAFT')
);
CREATE INDEX idx_mail_messages_thread ON mail_messages(thread_id,created_at,message_id);
CREATE INDEX idx_mail_messages_search ON mail_messages USING GIN(search_vector);

CREATE TABLE mail_recipients (
    message_id BIGINT NOT NULL REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    recipient_mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    recipient_type TEXT NOT NULL CHECK (recipient_type IN ('TO','CC','BCC')),
    ordinal SMALLINT NOT NULL CHECK (ordinal BETWEEN 1 AND 100),
    PRIMARY KEY(message_id,recipient_mailbox_id,recipient_type),
    UNIQUE(message_id,recipient_type,ordinal)
);
CREATE INDEX idx_mail_recipients_mailbox ON mail_recipients(recipient_mailbox_id,message_id);

CREATE TABLE mail_items (
    mail_item_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    message_id BIGINT NOT NULL REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    folder_id BIGINT NOT NULL REFERENCES mail_folders(folder_id) ON DELETE RESTRICT,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    is_starred BOOLEAN NOT NULL DEFAULT FALSE,
    trashed_from_kind TEXT CHECK (trashed_from_kind IS NULL OR trashed_from_kind IN ('INBOX','SENT','DRAFTS','SPAM')),
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(mailbox_id,message_id),
    CHECK ((deleted_at IS NULL) OR trashed_from_kind IS NOT NULL)
);
CREATE INDEX idx_mail_items_folder ON mail_items(mailbox_id,folder_id,created_at DESC,mail_item_id DESC);
CREATE INDEX idx_mail_items_unread ON mail_items(mailbox_id,created_at DESC) WHERE is_read=FALSE;

CREATE OR REPLACE FUNCTION mail_item_folder_owned() RETURNS trigger AS $$
DECLARE owner_id BIGINT; folder_kind TEXT;
BEGIN
    SELECT mailbox_id,kind INTO owner_id,folder_kind FROM mail_folders WHERE folder_id=NEW.folder_id;
    IF owner_id IS DISTINCT FROM NEW.mailbox_id THEN
        RAISE EXCEPTION 'mail folder tenant mismatch';
    END IF;
    IF NEW.deleted_at IS NOT NULL AND folder_kind <> 'TRASH' THEN
        RAISE EXCEPTION 'deleted mail item must be in trash';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_mail_item_folder_owned BEFORE INSERT OR UPDATE OF mailbox_id,folder_id,deleted_at ON mail_items FOR EACH ROW EXECUTE FUNCTION mail_item_folder_owned();

CREATE TABLE mail_attachment_blobs (
    blob_id UUID PRIMARY KEY,
    storage_key UUID NOT NULL UNIQUE,
    sha256 CHAR(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    byte_size BIGINT NOT NULL CHECK (byte_size BETWEEN 1 AND 26214400),
    content_type TEXT NOT NULL CHECK (length(content_type) BETWEEN 1 AND 255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_mail_attachment_blobs_sha ON mail_attachment_blobs(sha256,byte_size);

CREATE TABLE mail_attachments (
    attachment_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    blob_id UUID NOT NULL REFERENCES mail_attachment_blobs(blob_id) ON DELETE RESTRICT,
    original_filename TEXT NOT NULL CHECK (length(original_filename) BETWEEN 1 AND 255),
    ordinal SMALLINT NOT NULL CHECK (ordinal BETWEEN 1 AND 20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(message_id,ordinal),
    CHECK (position('/' in original_filename)=0),
    CHECK (position('\\' in original_filename)=0),
    CHECK (position(chr(0) in original_filename)=0),
    CHECK (original_filename NOT IN ('.','..'))
);

CREATE TABLE mail_action_buckets (
    mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('SEND','RECIPIENT','UPLOAD_BYTES')),
    bucket_start TIMESTAMPTZ NOT NULL,
    value BIGINT NOT NULL DEFAULT 0 CHECK (value >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(mailbox_id,action,bucket_start)
);
CREATE INDEX idx_mail_action_buckets_retention ON mail_action_buckets(bucket_start);

CREATE TABLE mail_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    message_id BIGINT REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL CHECK (event_type IN ('DRAFT_CREATE','DRAFT_UPDATE','SEND','READ','STAR','TRASH','RESTORE','SPAM','UNSPAM','ATTACH','DETACH')),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_mail_events_mailbox ON mail_events(mailbox_id,created_at DESC,event_id DESC);
