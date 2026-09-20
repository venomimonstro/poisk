CREATE TABLE organization_sources (
    source_key TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    trust_level SMALLINT NOT NULL DEFAULT 50 CHECK (trust_level BETWEEN 0 AND 100),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (source_key ~ '^[a-z0-9][a-z0-9._-]{0,63}$')
);

CREATE TABLE organization_import_batches (
    batch_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_key TEXT NOT NULL REFERENCES organization_sources(source_key) ON DELETE RESTRICT,
    external_batch_key TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'DRY_RUN' CHECK (mode IN ('DRY_RUN','APPLY')),
    status TEXT NOT NULL DEFAULT 'STAGING' CHECK (status IN ('STAGING','PLANNING','PLANNED','APPLYING','DONE','FAILED','CANCELLED')),
    row_count BIGINT NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    staged_count BIGINT NOT NULL DEFAULT 0 CHECK (staged_count >= 0),
    rejected_count BIGINT NOT NULL DEFAULT 0 CHECK (rejected_count >= 0),
    applied_count BIGINT NOT NULL DEFAULT 0 CHECK (applied_count >= 0),
    checkpoint_row BIGINT NOT NULL DEFAULT 0 CHECK (checkpoint_row >= 0),
    worker_id TEXT,
    lease_until TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(source_key, external_batch_key),
    CHECK ((status = 'APPLYING' AND worker_id IS NOT NULL AND lease_until IS NOT NULL) OR status <> 'APPLYING')
);
CREATE INDEX idx_org_import_batches_status ON organization_import_batches(status, updated_at, batch_id);
CREATE INDEX idx_org_import_batches_lease ON organization_import_batches(lease_until) WHERE status='APPLYING';

CREATE TABLE organization_staging_rows (
    staging_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES organization_import_batches(batch_id) ON DELETE CASCADE,
    source_key TEXT NOT NULL REFERENCES organization_sources(source_key) ON DELETE RESTRICT,
    source_record_id TEXT NOT NULL,
    source_row_number BIGINT NOT NULL CHECK (source_row_number > 0),
    raw_payload JSONB NOT NULL,
    raw_bytes INTEGER NOT NULL CHECK (raw_bytes > 0 AND raw_bytes <= 65536),
    normalized_name TEXT,
    normalized_phone TEXT,
    normalized_website TEXT,
    normalized_address TEXT,
    category_key TEXT,
    latitude DOUBLE PRECISION CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
    state TEXT NOT NULL DEFAULT 'STAGED' CHECK (state IN ('STAGED','VALID','REJECTED','PLANNED','APPLIED')),
    rejection_code TEXT,
    rejection_detail TEXT,
    payload_hash CHAR(64) NOT NULL CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(batch_id, source_record_id),
    UNIQUE(batch_id, source_row_number),
    CHECK ((state='REJECTED' AND rejection_code IS NOT NULL) OR state<>'REJECTED'),
    CHECK (rejection_detail IS NULL OR length(rejection_detail) <= 1024)
);
CREATE INDEX idx_org_staging_batch_state ON organization_staging_rows(batch_id, state, staging_id);
CREATE INDEX idx_org_staging_source_identity ON organization_staging_rows(source_key, source_record_id);

CREATE TABLE organizations (
    place_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','CLOSED','HIDDEN','REVIEW')),
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    category_key TEXT,
    phone TEXT,
    website TEXT,
    address TEXT,
    normalized_address TEXT,
    latitude DOUBLE PRECISION CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (quality_score BETWEEN 0 AND 100),
    source_count INTEGER NOT NULL DEFAULT 0 CHECK (source_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((latitude IS NULL) = (longitude IS NULL))
);
CREATE INDEX idx_organizations_name ON organizations(normalized_name, place_id);
CREATE INDEX idx_organizations_phone ON organizations(phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_organizations_website ON organizations(website) WHERE website IS NOT NULL;
CREATE INDEX idx_organizations_status ON organizations(status, place_id);

CREATE TABLE organization_source_links (
    source_key TEXT NOT NULL REFERENCES organization_sources(source_key) ON DELETE RESTRICT,
    source_record_id TEXT NOT NULL,
    place_id BIGINT NOT NULL REFERENCES organizations(place_id) ON DELETE RESTRICT,
    source_payload_hash CHAR(64) NOT NULL CHECK (source_payload_hash ~ '^[0-9a-f]{64}$'),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(source_key, source_record_id)
);
CREATE INDEX idx_org_source_links_place ON organization_source_links(place_id, source_key);

CREATE TABLE organization_import_plans (
    plan_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES organization_import_batches(batch_id) ON DELETE CASCADE,
    staging_id BIGINT NOT NULL REFERENCES organization_staging_rows(staging_id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('CREATE','UPDATE','NOOP','REJECT','REVIEW')),
    target_place_id BIGINT REFERENCES organizations(place_id) ON DELETE RESTRICT,
    match_rule TEXT,
    confidence SMALLINT NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 100),
    reason_code TEXT,
    plan_hash CHAR(64) NOT NULL CHECK (plan_hash ~ '^[0-9a-f]{64}$'),
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(batch_id, staging_id),
    CHECK ((action IN ('UPDATE','NOOP') AND target_place_id IS NOT NULL) OR action NOT IN ('UPDATE','NOOP')),
    CHECK ((action IN ('REJECT','REVIEW') AND reason_code IS NOT NULL) OR action NOT IN ('REJECT','REVIEW'))
);
CREATE INDEX idx_org_import_plans_batch_action ON organization_import_plans(batch_id, action, plan_id);

CREATE TABLE organization_merge_review (
    review_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES organization_import_batches(batch_id) ON DELETE CASCADE,
    staging_id BIGINT NOT NULL REFERENCES organization_staging_rows(staging_id) ON DELETE CASCADE,
    candidate_place_id BIGINT NOT NULL REFERENCES organizations(place_id) ON DELETE RESTRICT,
    reason_code TEXT NOT NULL,
    score SMALLINT NOT NULL CHECK (score BETWEEN 0 AND 100),
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','MERGED','CREATE_NEW','REJECTED')),
    decided_by TEXT,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(staging_id, candidate_place_id),
    CHECK ((status='OPEN' AND decided_at IS NULL) OR status<>'OPEN')
);
CREATE INDEX idx_org_merge_review_open ON organization_merge_review(status, created_at, review_id) WHERE status='OPEN';

CREATE TABLE organization_import_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_id BIGINT REFERENCES organization_import_batches(batch_id) ON DELETE CASCADE,
    staging_id BIGINT REFERENCES organization_staging_rows(staging_id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_org_import_events_batch ON organization_import_events(batch_id, event_id);
