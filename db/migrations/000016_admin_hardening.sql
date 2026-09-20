CREATE TABLE admin_users (
    admin_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('SUPERADMIN','OPERATOR','ANALYST','SUPPORT')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    totp_secret_cipher BYTEA,
    totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    failed_logins INTEGER NOT NULL DEFAULT 0 CHECK (failed_logins >= 0),
    locked_until TIMESTAMPTZ,
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(email) BETWEEN 3 AND 320),
    CHECK (length(password_hash) BETWEEN 40 AND 512),
    CHECK ((totp_enabled = FALSE) OR totp_secret_cipher IS NOT NULL)
);
CREATE UNIQUE INDEX uq_admin_users_email_lower ON admin_users(lower(email));
CREATE INDEX idx_admin_users_status ON admin_users(status, admin_id);

CREATE TABLE admin_sessions (
    session_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id BIGINT NOT NULL REFERENCES admin_users(admin_id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    csrf_hash BYTEA NOT NULL CHECK (octet_length(csrf_hash)=32),
    user_agent_hash BYTEA CHECK (user_agent_hash IS NULL OR octet_length(user_agent_hash)=32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);
CREATE INDEX idx_admin_sessions_active ON admin_sessions(admin_id, expires_at DESC)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_admin_sessions_expiry ON admin_sessions(expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE admin_recovery_codes (
    admin_id BIGINT NOT NULL REFERENCES admin_users(admin_id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL CHECK (octet_length(code_hash)=32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at TIMESTAMPTZ,
    PRIMARY KEY(admin_id, code_hash)
);
CREATE INDEX idx_admin_recovery_unused ON admin_recovery_codes(admin_id, created_at)
    WHERE used_at IS NULL;

CREATE TABLE admin_security_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id BIGINT REFERENCES admin_users(admin_id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_admin_security_events_time ON admin_security_events(created_at DESC, event_id DESC);
CREATE INDEX idx_admin_security_events_admin ON admin_security_events(admin_id, created_at DESC) WHERE admin_id IS NOT NULL;

CREATE OR REPLACE FUNCTION cap_admin_sessions() RETURNS trigger AS $$
BEGIN
    DELETE FROM admin_sessions
    WHERE admin_id = NEW.admin_id AND (expires_at <= now() OR revoked_at IS NOT NULL);

    IF (SELECT count(*) FROM admin_sessions WHERE admin_id = NEW.admin_id AND revoked_at IS NULL AND expires_at > now()) >= 8 THEN
        DELETE FROM admin_sessions
        WHERE session_id = (
            SELECT session_id FROM admin_sessions
            WHERE admin_id = NEW.admin_id AND revoked_at IS NULL AND expires_at > now()
            ORDER BY last_seen_at ASC, session_id ASC
            LIMIT 1
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_cap_admin_sessions
BEFORE INSERT ON admin_sessions
FOR EACH ROW EXECUTE FUNCTION cap_admin_sessions();
