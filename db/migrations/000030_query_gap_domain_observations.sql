CREATE TABLE query_gap_domain_observations (
    query_hash BYTEA NOT NULL CHECK (octet_length(query_hash)=32),
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    seen_buckets SMALLINT NOT NULL DEFAULT 1 CHECK (seen_buckets BETWEEN 1 AND 144),
    PRIMARY KEY(query_hash,domain_id)
);

CREATE INDEX idx_query_gap_domain_observations_domain
    ON query_gap_domain_observations(domain_id,last_seen_at DESC);
