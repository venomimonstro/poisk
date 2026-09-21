-- Repair historical duplicate numeric migration versions 000018/000019.
-- This migration is intentionally idempotent so both fresh installs and databases
-- that applied either duplicate variant converge to the same final schema.

CREATE TABLE IF NOT EXISTS admin_action_previews (
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
CREATE INDEX IF NOT EXISTS idx_admin_action_previews_active
    ON admin_action_previews(admin_id,session_id,expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS admin_mutation_previews (
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
CREATE INDEX IF NOT EXISTS idx_admin_mutation_preview_active
    ON admin_mutation_previews(admin_id,expires_at)
    WHERE used_at IS NULL;

ALTER TABLE app_releases ADD COLUMN IF NOT EXISTS backend_image TEXT;
ALTER TABLE app_releases ADD COLUMN IF NOT EXISTS frontend_image TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid='app_releases'::regclass AND conname='chk_release_backend_image'
    ) THEN
        ALTER TABLE app_releases ADD CONSTRAINT chk_release_backend_image
            CHECK (backend_image IS NULL OR (length(backend_image) BETWEEN 3 AND 512 AND position(' ' in backend_image)=0));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid='app_releases'::regclass AND conname='chk_release_frontend_image'
    ) THEN
        ALTER TABLE app_releases ADD CONSTRAINT chk_release_frontend_image
            CHECK (frontend_image IS NULL OR (length(frontend_image) BETWEEN 3 AND 512 AND position(' ' in frontend_image)=0));
    END IF;
END $$;

INSERT INTO system_settings(key,value,updated_at)
VALUES('resource_pressure','{"state":"NORMAL","disk_used_pct":0,"memory_used_pct":0,"checked_at":null}'::jsonb,now())
ON CONFLICT(key) DO NOTHING;
