CREATE TABLE admin_mutation_previews (
    preview_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id BIGINT NOT NULL REFERENCES admin_users(admin_id) ON DELETE CASCADE,
    preview_token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(preview_token_hash)=32),
    mutation_type TEXT NOT NULL CHECK (mutation_type IN ('DOMAIN_CONTROL')),
    payload JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX idx_admin_mutation_preview_active
    ON admin_mutation_previews(admin_id,expires_at)
    WHERE used_at IS NULL;
