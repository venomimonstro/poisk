ALTER TABLE consumer_users
    ADD COLUMN failed_login_count SMALLINT NOT NULL DEFAULT 0 CHECK (failed_login_count BETWEEN 0 AND 20),
    ADD COLUMN locked_until TIMESTAMPTZ;

CREATE INDEX idx_consumer_users_locked_until
    ON consumer_users(locked_until)
    WHERE locked_until IS NOT NULL;
