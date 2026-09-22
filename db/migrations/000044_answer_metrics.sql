CREATE TABLE answer_metrics_hourly (
    bucket_start TIMESTAMPTZ PRIMARY KEY,
    requests BIGINT NOT NULL DEFAULT 0 CHECK (requests >= 0),
    available BIGINT NOT NULL DEFAULT 0 CHECK (available >= 0),
    fallback_low_confidence BIGINT NOT NULL DEFAULT 0 CHECK (fallback_low_confidence >= 0),
    fallback_insufficient_sources BIGINT NOT NULL DEFAULT 0 CHECK (fallback_insufficient_sources >= 0),
    fallback_other BIGINT NOT NULL DEFAULT 0 CHECK (fallback_other >= 0),
    errors BIGINT NOT NULL DEFAULT 0 CHECK (errors >= 0),
    confidence_sum DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence_sum >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (date_trunc('hour',bucket_start)=bucket_start)
);
