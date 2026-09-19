ALTER TABLE domains
    ADD COLUMN authority_score DOUBLE PRECISION NOT NULL DEFAULT 0
    CHECK (authority_score BETWEEN 0 AND 100);

CREATE TABLE domain_edges (
    source_domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    target_domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (weight > 0 AND weight <= 1000000),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (source_domain_id, target_domain_id),
    CHECK (source_domain_id <> target_domain_id)
);

CREATE INDEX idx_domain_edges_target
    ON domain_edges(target_domain_id, source_domain_id);

CREATE INDEX idx_domains_authority
    ON domains(authority_score DESC, domain_id ASC)
    WHERE status = 'ACTIVE' AND policy IN ('ALLOW','LIMITED');
