CREATE TABLE organization_web_links (
    place_id BIGINT NOT NULL REFERENCES organizations(place_id) ON DELETE CASCADE,
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    source_url_id BIGINT REFERENCES urls(url_id) ON DELETE SET NULL,
    match_type TEXT NOT NULL CHECK (match_type IN ('WEBSITE_HOST','SCHEMA_URL','SCHEMA_PHONE')),
    confidence SMALLINT NOT NULL CHECK (confidence BETWEEN 0 AND 100),
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (place_id, domain_id, match_type)
);

CREATE INDEX idx_org_web_links_domain
    ON organization_web_links(domain_id, confidence DESC, place_id);

CREATE INDEX idx_org_web_links_place
    ON organization_web_links(place_id, confidence DESC, domain_id);
