CREATE TABLE organization_review_action_buckets (
    consumer_user_id BIGINT NOT NULL REFERENCES consumer_users(user_id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('WRITE','REPORT','REPLY')),
    bucket_start TIMESTAMPTZ NOT NULL,
    hits SMALLINT NOT NULL DEFAULT 0 CHECK (hits BETWEEN 0 AND 20),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(consumer_user_id,action,bucket_start),
    CHECK (date_trunc('hour',bucket_start)=bucket_start)
);
CREATE INDEX idx_review_action_buckets_retention ON organization_review_action_buckets(bucket_start);
