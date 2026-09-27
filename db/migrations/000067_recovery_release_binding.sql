ALTER TABLE recovery_drills
    ADD COLUMN git_commit CHAR(40);

ALTER TABLE recovery_drills
    ADD CONSTRAINT recovery_drills_git_commit_check
    CHECK (git_commit IS NULL OR git_commit ~ '^[0-9a-f]{40}$') NOT VALID;

-- Historical rows predate release binding and remain readable, but readiness
-- deliberately ignores them. New PASS rows are required to carry a commit by
-- the repository layer and commercial gate.
CREATE INDEX idx_recovery_drills_release_latest
    ON recovery_drills(drill_type,git_commit,database_schema,completed_at DESC,drill_id DESC)
    WHERE git_commit IS NOT NULL;
