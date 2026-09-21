CREATE TABLE agencies (
    agency_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 2 AND 160),
    public_code TEXT NOT NULL UNIQUE CHECK (public_code ~ '^ag_[A-Za-z0-9_-]{16,64}$'),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    created_by_user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agency_members (
    agency_id BIGINT NOT NULL REFERENCES agencies(agency_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('OWNER','MANAGER','ANALYST')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','REVOKED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (agency_id,user_id)
);
CREATE INDEX idx_agency_members_user_active ON agency_members(user_id,agency_id) WHERE status='ACTIVE';

CREATE TABLE agency_site_access (
    agency_id BIGINT NOT NULL REFERENCES agencies(agency_id) ON DELETE CASCADE,
    site_id BIGINT NOT NULL REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('READ','MANAGE')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','REVOKED')),
    granted_by_user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE RESTRICT,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agency_id,site_id)
);
CREATE INDEX idx_agency_site_access_site_active ON agency_site_access(site_id,agency_id) WHERE status='ACTIVE';

CREATE TABLE agency_audit_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    agency_id BIGINT NOT NULL REFERENCES agencies(agency_id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE RESTRICT,
    site_id BIGINT REFERENCES webmaster_sites(site_id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (length(action) BETWEEN 2 AND 96),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_agency_audit_events_agency_created ON agency_audit_events(agency_id,created_at DESC);
