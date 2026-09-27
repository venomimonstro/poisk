ALTER TABLE quality_runs
    ADD COLUMN git_commit CHAR(40),
    ADD COLUMN database_schema BIGINT;

ALTER TABLE quality_runs
    ADD CONSTRAINT quality_runs_git_commit_check
    CHECK (git_commit IS NULL OR git_commit ~ '^[0-9a-f]{40}$'),
    ADD CONSTRAINT quality_runs_database_schema_check
    CHECK (database_schema IS NULL OR database_schema > 0),
    ADD CONSTRAINT quality_runs_release_binding_pair_check
    CHECK ((git_commit IS NULL) = (database_schema IS NULL));

CREATE INDEX idx_quality_runs_release_latest
    ON quality_runs(git_commit,database_schema,completed_at DESC,run_id DESC)
    WHERE git_commit IS NOT NULL;
