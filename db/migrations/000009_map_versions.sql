CREATE TABLE map_versions (
    map_version TEXT PRIMARY KEY,
    pmtiles_path TEXT NOT NULL UNIQUE,
    style_path TEXT NOT NULL UNIQUE,
    pmtiles_sha256 CHAR(64) NOT NULL CHECK (pmtiles_sha256 ~ '^[0-9a-f]{64}$'),
    style_sha256 CHAR(64) NOT NULL CHECK (style_sha256 ~ '^[0-9a-f]{64}$'),
    pmtiles_size BIGINT NOT NULL CHECK (pmtiles_size > 0),
    style_size BIGINT NOT NULL CHECK (style_size > 0),
    min_lon DOUBLE PRECISION NOT NULL CHECK (min_lon BETWEEN -180 AND 180),
    min_lat DOUBLE PRECISION NOT NULL CHECK (min_lat BETWEEN -90 AND 90),
    max_lon DOUBLE PRECISION NOT NULL CHECK (max_lon BETWEEN -180 AND 180),
    max_lat DOUBLE PRECISION NOT NULL CHECK (max_lat BETWEEN -90 AND 90),
    min_zoom SMALLINT NOT NULL CHECK (min_zoom BETWEEN 0 AND 24),
    max_zoom SMALLINT NOT NULL CHECK (max_zoom BETWEEN 0 AND 24),
    center_lon DOUBLE PRECISION NOT NULL CHECK (center_lon BETWEEN -180 AND 180),
    center_lat DOUBLE PRECISION NOT NULL CHECK (center_lat BETWEEN -90 AND 90),
    center_zoom DOUBLE PRECISION NOT NULL CHECK (center_zoom BETWEEN 0 AND 24),
    status TEXT NOT NULL DEFAULT 'VALIDATED' CHECK (status IN ('VALIDATED','REJECTED','RETIRED')),
    source_name TEXT NOT NULL DEFAULT 'OpenStreetMap',
    attribution_html TEXT NOT NULL DEFAULT '© OpenStreetMap contributors',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (min_lon < max_lon),
    CHECK (min_lat < max_lat),
    CHECK (min_zoom <= max_zoom),
    CHECK (pmtiles_path ~ '^[A-Za-z0-9][A-Za-z0-9._/-]*\.pmtiles$'),
    CHECK (style_path ~ '^[A-Za-z0-9][A-Za-z0-9._/-]*\.json$'),
    CHECK (position('..' in pmtiles_path) = 0),
    CHECK (position('..' in style_path) = 0),
    CHECK (left(pmtiles_path,1) <> '/'),
    CHECK (left(style_path,1) <> '/')
);

CREATE TABLE map_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    active_version TEXT REFERENCES map_versions(map_version) ON DELETE RESTRICT,
    previous_version TEXT REFERENCES map_versions(map_version) ON DELETE RESTRICT,
    activated_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO map_state(singleton) VALUES(TRUE) ON CONFLICT(singleton) DO NOTHING;

CREATE TABLE map_version_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    action TEXT NOT NULL CHECK (action IN ('REGISTER','ACTIVATE','ROLLBACK','REJECT','RETIRE')),
    map_version TEXT NOT NULL,
    from_version TEXT,
    actor TEXT NOT NULL DEFAULT 'SYSTEM',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_map_version_events_created ON map_version_events(created_at DESC,event_id DESC);
