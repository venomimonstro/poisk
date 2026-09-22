CREATE TABLE recovery_drills (
    drill_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    drill_type TEXT NOT NULL CHECK (drill_type IN ('BACKUP','RESTORE')),
    status TEXT NOT NULL CHECK (status IN ('PASS','FAIL')),
    artifact_ref TEXT CHECK (artifact_ref IS NULL OR length(artifact_ref) BETWEEN 1 AND 240),
    artifact_sha256 CHAR(64) CHECK (artifact_sha256 IS NULL OR artifact_sha256 ~ '^[0-9a-f]{64}$'),
    artifact_bytes BIGINT CHECK (artifact_bytes IS NULL OR artifact_bytes >= 0),
    duration_ms BIGINT CHECK (duration_ms IS NULL OR duration_ms >= 0),
    database_schema BIGINT NOT NULL CHECK (database_schema >= 0),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    actor TEXT NOT NULL CHECK (length(actor) BETWEEN 1 AND 120),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_recovery_drills_latest ON recovery_drills(drill_type,completed_at DESC,drill_id DESC);

CREATE OR REPLACE FUNCTION recovery_drills_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'recovery_drills is immutable';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_recovery_drills_no_update BEFORE UPDATE OR DELETE ON recovery_drills FOR EACH ROW EXECUTE FUNCTION recovery_drills_immutable();
