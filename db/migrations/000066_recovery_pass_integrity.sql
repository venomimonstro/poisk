ALTER TABLE recovery_drills
    ADD CONSTRAINT recovery_drills_pass_evidence_check
    CHECK (
        status <> 'PASS' OR (
            artifact_ref IS NOT NULL AND length(artifact_ref) BETWEEN 1 AND 240 AND
            artifact_sha256 IS NOT NULL AND artifact_sha256 ~ '^[0-9a-f]{64}$' AND
            artifact_bytes IS NOT NULL AND artifact_bytes > 0 AND
            duration_ms IS NOT NULL AND duration_ms > 0
        )
    ) NOT VALID;

-- NOT VALID deliberately preserves historical drills created before this
-- invariant existed. PostgreSQL still enforces the constraint for new and
-- updated rows; readiness also rejects unverifiable historical PASS rows.
