ALTER TABLE quality_runs
    ADD COLUMN release_version TEXT;

ALTER TABLE quality_runs
    ADD CONSTRAINT quality_runs_release_version_check
    CHECK (
        release_version IS NULL OR
        (length(release_version) BETWEEN 1 AND 128 AND release_version !~ '[[:cntrl:]]')
    ) NOT VALID;

CREATE INDEX idx_quality_runs_candidate_latest
    ON quality_runs(
        release_version,
        git_commit,
        database_schema,
        completed_at DESC,
        run_id DESC
    )
    WHERE release_version IS NOT NULL AND git_commit IS NOT NULL;

-- Historical quality results remain unbound and therefore cannot certify a
-- new release candidate in the commercial readiness gate.
