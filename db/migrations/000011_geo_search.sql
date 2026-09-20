ALTER TABLE organization_staging_rows
    ADD COLUMN city_key TEXT;

ALTER TABLE organization_staging_rows
    ADD CONSTRAINT chk_org_staging_city_key
    CHECK (city_key IS NULL OR city_key ~ '^[a-z0-9][a-z0-9._-]{0,95}$');

ALTER TABLE organizations
    ADD COLUMN city_key TEXT;

ALTER TABLE organizations
    ADD CONSTRAINT chk_organizations_city_key
    CHECK (city_key IS NULL OR city_key ~ '^[a-z0-9][a-z0-9._-]{0,95}$');

ALTER TABLE organizations
    ADD COLUMN location geography(Point,4326)
    GENERATED ALWAYS AS (
        CASE
            WHEN latitude IS NOT NULL AND longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(longitude, latitude),4326)::geography
            ELSE NULL
        END
    ) STORED;

CREATE INDEX idx_organizations_city_category
    ON organizations(city_key, category_key, place_id)
    WHERE status IN ('ACTIVE','REVIEW');

CREATE INDEX idx_organizations_location_gist
    ON organizations USING GIST(location)
    WHERE location IS NOT NULL AND status IN ('ACTIVE','REVIEW');
