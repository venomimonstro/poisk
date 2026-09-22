CREATE TABLE mail_blob_gc (
    storage_key UUID PRIMARY KEY,
    byte_size BIGINT NOT NULL CHECK (byte_size > 0),
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT CHECK (last_error IS NULL OR length(last_error) <= 1000),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_mail_blob_gc_ready ON mail_blob_gc(next_attempt_at,queued_at);
