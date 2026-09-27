ALTER TABLE commercial_readiness_evidence
    ADD COLUMN release_version TEXT;

ALTER TABLE commercial_readiness_evidence
    ADD CONSTRAINT commercial_readiness_evidence_release_version_check
    CHECK (
        release_version IS NULL OR
        (length(release_version) BETWEEN 1 AND 128 AND release_version !~ '[[:cntrl:]]')
    );

CREATE INDEX idx_commercial_readiness_release_latest
    ON commercial_readiness_evidence(
        evidence_type,
        release_version,
        git_commit,
        database_schema,
        completed_at DESC,
        evidence_id DESC
    );

-- Historical evidence intentionally remains NULL. The commercial gate requires
-- an exact release_version match, so evidence created before this migration
-- cannot certify a newly staged release candidate.
