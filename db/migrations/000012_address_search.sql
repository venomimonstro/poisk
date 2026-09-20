CREATE TABLE address_import_batches (
    batch_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_revision TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'STAGING' CHECK (status IN ('STAGING','RESOLVING','APPLYING','DONE','FAILED')),
    checkpoint_file TEXT,
    checkpoint_row BIGINT NOT NULL DEFAULT 0 CHECK (checkpoint_row >= 0),
    staged_count BIGINT NOT NULL DEFAULT 0,
    rejected_count BIGINT NOT NULL DEFAULT 0,
    applied_count BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE address_staging_rows (
    staging_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES address_import_batches(batch_id) ON DELETE CASCADE,
    source_file TEXT NOT NULL,
    source_row BIGINT NOT NULL CHECK (source_row > 0),
    record_kind TEXT NOT NULL CHECK (record_kind IN ('ADDR_OBJ','HOUSE','HIERARCHY')),
    region_code SMALLINT NOT NULL CHECK (region_code BETWEEN 1 AND 99),
    object_id BIGINT NOT NULL CHECK (object_id > 0),
    object_guid UUID,
    parent_object_id BIGINT CHECK (parent_object_id > 0),
    level SMALLINT CHECK (level BETWEEN 0 AND 99),
    name TEXT,
    type_name TEXT,
    house_num TEXT,
    add_num1 TEXT,
    add_num2 TEXT,
    is_actual BOOLEAN,
    is_active BOOLEAN,
    payload_hash CHAR(64) NOT NULL CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
    raw_payload JSONB NOT NULL,
    state TEXT NOT NULL DEFAULT 'VALID' CHECK (state IN ('VALID','REJECTED','APPLIED')),
    rejection_code TEXT,
    rejection_detail TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(batch_id,source_file,source_row),
    UNIQUE(batch_id,record_kind,region_code,object_id)
);
CREATE INDEX idx_address_staging_resolve ON address_staging_rows(batch_id,record_kind,region_code,object_id) WHERE state='VALID';
CREATE INDEX idx_address_staging_parent ON address_staging_rows(batch_id,region_code,parent_object_id) WHERE parent_object_id IS NOT NULL AND state='VALID';

CREATE TABLE addresses (
    address_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    region_code SMALLINT NOT NULL CHECK (region_code BETWEEN 1 AND 99),
    gar_object_id BIGINT NOT NULL CHECK (gar_object_id > 0),
    object_guid UUID,
    object_kind TEXT NOT NULL CHECK (object_kind IN ('ADDR_OBJ','HOUSE')),
    level SMALLINT CHECK (level BETWEEN 0 AND 99),
    parent_address_id BIGINT REFERENCES addresses(address_id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    type_name TEXT,
    house_num TEXT,
    normalized_name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    full_address TEXT NOT NULL,
    postal_code TEXT,
    latitude DOUBLE PRECISION CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION CHECK (longitude BETWEEN -180 AND 180),
    location geography(Point,4326) GENERATED ALWAYS AS (
        CASE WHEN latitude IS NOT NULL AND longitude IS NOT NULL
             THEN ST_SetSRID(ST_MakePoint(longitude,latitude),4326)::geography
             ELSE NULL END
    ) STORED,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE')),
    source_revision TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(region_code,gar_object_id),
    CHECK ((latitude IS NULL) = (longitude IS NULL))
);
CREATE UNIQUE INDEX uq_addresses_guid ON addresses(object_guid) WHERE object_guid IS NOT NULL;
CREATE INDEX idx_addresses_parent ON addresses(parent_address_id,address_id);
CREATE INDEX idx_addresses_region_level ON addresses(region_code,level,address_id) WHERE status='ACTIVE';
CREATE INDEX idx_addresses_location_gist ON addresses USING GIST(location) WHERE location IS NOT NULL AND status='ACTIVE';

CREATE TABLE address_import_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES address_import_batches(batch_id) ON DELETE CASCADE,
    staging_id BIGINT REFERENCES address_staging_rows(staging_id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_address_import_events_batch ON address_import_events(batch_id,event_id);
