CREATE TABLE query_signal_buckets (
    query_hash BYTEA NOT NULL CHECK (octet_length(query_hash)=32),
    bucket_start TIMESTAMPTZ NOT NULL,
    hits SMALLINT NOT NULL DEFAULT 1 CHECK (hits BETWEEN 1 AND 3),
    zero_result_hits SMALLINT NOT NULL DEFAULT 0 CHECK (zero_result_hits BETWEEN 0 AND 3),
    low_quality_hits SMALLINT NOT NULL DEFAULT 0 CHECK (low_quality_hits BETWEEN 0 AND 3),
    low_freshness_hits SMALLINT NOT NULL DEFAULT 0 CHECK (low_freshness_hits BETWEEN 0 AND 3),
    high_spam_hits SMALLINT NOT NULL DEFAULT 0 CHECK (high_spam_hits BETWEEN 0 AND 3),
    result_count_sum INTEGER NOT NULL DEFAULT 0 CHECK (result_count_sum BETWEEN 0 AND 300),
    quality_sum INTEGER NOT NULL DEFAULT 0 CHECK (quality_sum BETWEEN 0 AND 300),
    freshness_sum INTEGER NOT NULL DEFAULT 0 CHECK (freshness_sum BETWEEN 0 AND 300),
    spam_sum INTEGER NOT NULL DEFAULT 0 CHECK (spam_sum BETWEEN 0 AND 300),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(query_hash,bucket_start),
    CHECK (date_trunc('minute',bucket_start)=bucket_start)
);
CREATE INDEX idx_query_signal_buckets_recent ON query_signal_buckets(bucket_start DESC);

CREATE TABLE query_gaps (
    gap_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    query_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(query_hash)=32),
    representative_query TEXT CHECK (representative_query IS NULL OR length(representative_query) BETWEEN 1 AND 256),
    state TEXT NOT NULL DEFAULT 'WATCH' CHECK (state IN ('WATCH','OPEN','RESOLVED','SUPPRESSED')),
    demand_score SMALLINT NOT NULL DEFAULT 0 CHECK (demand_score BETWEEN 0 AND 100),
    coverage_score SMALLINT NOT NULL DEFAULT 100 CHECK (coverage_score BETWEEN 0 AND 100),
    quality_score SMALLINT NOT NULL DEFAULT 100 CHECK (quality_score BETWEEN 0 AND 100),
    freshness_score SMALLINT NOT NULL DEFAULT 100 CHECK (freshness_score BETWEEN 0 AND 100),
    spam_score SMALLINT NOT NULL DEFAULT 0 CHECK (spam_score BETWEEN 0 AND 100),
    gap_score SMALLINT NOT NULL DEFAULT 0 CHECK (gap_score BETWEEN 0 AND 100),
    independent_buckets SMALLINT NOT NULL DEFAULT 0 CHECK (independent_buckets BETWEEN 0 AND 144),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    qualified_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    suppressed_at TIMESTAMPTZ,
    last_feedback_at TIMESTAMPTZ,
    feedback_count INTEGER NOT NULL DEFAULT 0 CHECK (feedback_count BETWEEN 0 AND 1000)
);
CREATE INDEX idx_query_gaps_open ON query_gaps(gap_score DESC,last_seen_at DESC,gap_id) WHERE state='OPEN';

CREATE TABLE query_gap_domain_feedback (
    gap_id BIGINT NOT NULL REFERENCES query_gaps(gap_id) ON DELETE CASCADE,
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    boost SMALLINT NOT NULL CHECK (boost BETWEEN 1 AND 25),
    expires_at TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN ('LOW_COVERAGE','LOW_QUALITY','LOW_FRESHNESS')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(gap_id,domain_id),
    CHECK (expires_at > created_at)
);
CREATE INDEX idx_query_gap_domain_feedback_active ON query_gap_domain_feedback(domain_id,expires_at,boost DESC);

CREATE TABLE query_gap_feedback_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    gap_id BIGINT NOT NULL REFERENCES query_gaps(gap_id) ON DELETE CASCADE,
    domain_id BIGINT REFERENCES domains(domain_id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('OPEN','REFRESH','BOOST_DOMAIN','EXPIRE_BOOST','RESOLVE','SUPPRESS')),
    boost SMALLINT CHECK (boost IS NULL OR boost BETWEEN 1 AND 25),
    idempotency_key TEXT NOT NULL UNIQUE CHECK (length(idempotency_key) BETWEEN 12 AND 160),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_query_gap_feedback_events_gap ON query_gap_feedback_events(gap_id,event_id);
