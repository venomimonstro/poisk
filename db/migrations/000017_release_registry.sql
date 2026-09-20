CREATE TABLE app_releases (
    release_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version TEXT NOT NULL UNIQUE,
    build_sha TEXT NOT NULL,
    required_schema_version BIGINT NOT NULL CHECK (required_schema_version > 0),
    map_version TEXT,
    web_index_schema INTEGER NOT NULL CHECK (web_index_schema > 0),
    organization_index_schema INTEGER NOT NULL CHECK (organization_index_schema > 0),
    address_index_schema INTEGER NOT NULL CHECK (address_index_schema > 0),
    config_hash CHAR(64) NOT NULL CHECK (config_hash ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL DEFAULT 'STAGED' CHECK (status IN ('STAGED','ACTIVE','PREVIOUS','FAILED','RETIRED')),
    preflight_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_app_releases_status ON app_releases(status, release_id DESC);

CREATE TABLE release_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    active_release_id BIGINT REFERENCES app_releases(release_id) ON DELETE RESTRICT,
    previous_release_id BIGINT REFERENCES app_releases(release_id) ON DELETE RESTRICT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO release_state(singleton) VALUES(TRUE) ON CONFLICT(singleton) DO NOTHING;

CREATE TABLE release_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    release_id BIGINT REFERENCES app_releases(release_id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('STAGE','PREFLIGHT','ACTIVATE','ROLLBACK','FAIL')),
    actor TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_release_events_time ON release_events(created_at DESC,event_id DESC);
