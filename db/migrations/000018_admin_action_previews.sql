CREATE TABLE admin_action_previews (
    preview_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id BIGINT NOT NULL REFERENCES admin_users(admin_id) ON DELETE CASCADE,
    session_id BIGINT NOT NULL REFERENCES admin_sessions(session_id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    action_type TEXT NOT NULL CHECK (action_type IN ('DOMAIN_POLICY')),
    target_type TEXT NOT NULL CHECK (target_type IN ('DOMAIN')),
    target_id BIGINT NOT NULL CHECK (target_id>0),
    payload JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX idx_admin_action_previews_active
    ON admin_action_previews(admin_id,session_id,expires_at)
    WHERE consumed_at IS NULL;
