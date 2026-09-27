ALTER TABLE recovery_drills
    ADD COLUMN release_version TEXT;

ALTER TABLE recovery_drills
    ADD CONSTRAINT recovery_drills_release_version_check
    CHECK (
        release_version IS NULL OR
        (length(release_version) BETWEEN 1 AND 128 AND release_version !~ '[[:cntrl:]]')
    ) NOT VALID;

CREATE INDEX idx_recovery_drills_candidate_latest
    ON recovery_drills(
        drill_type,
        release_version,
        git_commit,
        database_schema,
        completed_at DESC,
        drill_id DESC
    )
    WHERE git_commit IS NOT NULL AND release_version IS NOT NULL;

-- Historical drills intentionally remain unbound and cannot satisfy the
-- final commercial readiness gate for a newly staged release candidate.
