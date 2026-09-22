CREATE TABLE datahub_pages (
    page_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_type TEXT NOT NULL CHECK (page_type IN ('CITY','CATEGORY','CITY_CATEGORY','ORGANIZATION','WEBSITE')),
    city_key TEXT,
    category_key TEXT,
    place_id BIGINT REFERENCES organizations(place_id) ON DELETE CASCADE,
    domain_id BIGINT REFERENCES domains(domain_id) ON DELETE CASCADE,
    slug TEXT NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9._~-]{0,190}$'),
    canonical_path TEXT NOT NULL CHECK (canonical_path ~ '^/data/[a-z0-9/_~.-]{1,240}$'),
    state TEXT NOT NULL DEFAULT 'DRAFT' CHECK (state IN ('DRAFT','PUBLISHED','SUPPRESSED')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    evidence_count INTEGER NOT NULL DEFAULT 0 CHECK (evidence_count >= 0),
    quality_score SMALLINT NOT NULL DEFAULT 0 CHECK (quality_score BETWEEN 0 AND 100),
    content_hash CHAR(64) CHECK (content_hash IS NULL OR content_hash ~ '^[0-9a-f]{64}$'),
    title TEXT,
    meta_description TEXT,
    published_at TIMESTAMPTZ,
    refreshed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (city_key IS NULL OR city_key ~ '^[a-z0-9][a-z0-9._-]{0,95}$'),
    CHECK (category_key IS NULL OR category_key ~ '^[a-z0-9][a-z0-9._-]{0,95}$'),
    CHECK (title IS NULL OR length(title) BETWEEN 3 AND 180),
    CHECK (meta_description IS NULL OR length(meta_description) BETWEEN 20 AND 320),
    CHECK ((state='PUBLISHED' AND published_at IS NOT NULL AND content_hash IS NOT NULL) OR state<>'PUBLISHED'),
    CHECK (
        (page_type='CITY' AND city_key IS NOT NULL AND category_key IS NULL AND place_id IS NULL AND domain_id IS NULL) OR
        (page_type='CATEGORY' AND city_key IS NULL AND category_key IS NOT NULL AND place_id IS NULL AND domain_id IS NULL) OR
        (page_type='CITY_CATEGORY' AND city_key IS NOT NULL AND category_key IS NOT NULL AND place_id IS NULL AND domain_id IS NULL) OR
        (page_type='ORGANIZATION' AND place_id IS NOT NULL AND city_key IS NULL AND category_key IS NULL AND domain_id IS NULL) OR
        (page_type='WEBSITE' AND domain_id IS NOT NULL AND city_key IS NULL AND category_key IS NULL AND place_id IS NULL)
    ),
    UNIQUE(page_type,slug),
    UNIQUE(canonical_path)
);

CREATE UNIQUE INDEX uq_datahub_city
    ON datahub_pages(city_key) WHERE page_type='CITY';
CREATE UNIQUE INDEX uq_datahub_category
    ON datahub_pages(category_key) WHERE page_type='CATEGORY';
CREATE UNIQUE INDEX uq_datahub_city_category
    ON datahub_pages(city_key,category_key) WHERE page_type='CITY_CATEGORY';
CREATE UNIQUE INDEX uq_datahub_organization
    ON datahub_pages(place_id) WHERE page_type='ORGANIZATION';
CREATE UNIQUE INDEX uq_datahub_website
    ON datahub_pages(domain_id) WHERE page_type='WEBSITE';
CREATE INDEX idx_datahub_published
    ON datahub_pages(page_type,updated_at DESC,page_id) WHERE state='PUBLISHED';

CREATE TABLE datahub_page_versions (
    page_id BIGINT NOT NULL REFERENCES datahub_pages(page_id) ON DELETE CASCADE,
    version BIGINT NOT NULL CHECK (version > 0),
    content_hash CHAR(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    evidence_count INTEGER NOT NULL CHECK (evidence_count >= 0),
    quality_score SMALLINT NOT NULL CHECK (quality_score BETWEEN 0 AND 100),
    snapshot JSONB NOT NULL CHECK (jsonb_typeof(snapshot)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(page_id,version)
);

CREATE TABLE datahub_trends_daily (
    day DATE NOT NULL,
    query_hash BYTEA NOT NULL CHECK (octet_length(query_hash)=32),
    representative_query TEXT NOT NULL CHECK (length(representative_query) BETWEEN 1 AND 256),
    demand_score SMALLINT NOT NULL CHECK (demand_score BETWEEN 0 AND 100),
    gap_score SMALLINT NOT NULL CHECK (gap_score BETWEEN 0 AND 100),
    independent_buckets SMALLINT NOT NULL CHECK (independent_buckets BETWEEN 3 AND 144),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(day,query_hash)
);
CREATE INDEX idx_datahub_trends_day_score ON datahub_trends_daily(day,demand_score DESC,gap_score DESC);

CREATE TABLE datahub_publication_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id BIGINT REFERENCES datahub_pages(page_id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('BUILD','PUBLISH','UNPUBLISH','SUPPRESS','ROLLBACK')),
    from_version BIGINT,
    to_version BIGINT,
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 2 AND 160),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (from_version IS NULL OR from_version > 0),
    CHECK (to_version IS NULL OR to_version > 0)
);
CREATE INDEX idx_datahub_publication_events_page ON datahub_publication_events(page_id,event_id);
