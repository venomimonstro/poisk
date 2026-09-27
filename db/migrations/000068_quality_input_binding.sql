ALTER TABLE quality_runs
    ADD COLUMN golden_sha256 CHAR(64),
    ADD COLUMN thresholds_sha256 CHAR(64);

ALTER TABLE quality_runs
    ADD CONSTRAINT quality_runs_input_hash_pair_check
    CHECK ((golden_sha256 IS NULL) = (thresholds_sha256 IS NULL)),
    ADD CONSTRAINT quality_runs_golden_sha256_check
    CHECK (golden_sha256 IS NULL OR golden_sha256 ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT quality_runs_thresholds_sha256_check
    CHECK (thresholds_sha256 IS NULL OR thresholds_sha256 ~ '^[0-9a-f]{64}$');

CREATE INDEX idx_quality_runs_release_inputs_latest
    ON quality_runs(git_commit,database_schema,golden_sha256,thresholds_sha256,completed_at DESC,run_id DESC)
    WHERE git_commit IS NOT NULL AND golden_sha256 IS NOT NULL;
