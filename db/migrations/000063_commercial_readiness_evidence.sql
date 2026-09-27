CREATE TABLE commercial_readiness_evidence (
    evidence_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN (
        'BUILD_UNIT','INTEGRATION','FRESH_INSTALL','UPGRADE','BROWSER_SMOKE','MTA_FLOW'
    )),
    status TEXT NOT NULL CHECK (status IN ('PASS','FAIL')),
    git_commit CHAR(40) NOT NULL CHECK (git_commit ~ '^[0-9a-f]{40}$'),
    database_schema BIGINT NOT NULL CHECK (database_schema > 0),
    artifact_ref TEXT NOT NULL CHECK (length(artifact_ref) BETWEEN 1 AND 240),
    artifact_sha256 CHAR(64) NOT NULL CHECK (artifact_sha256 ~ '^[0-9a-f]{64}$'),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    actor TEXT NOT NULL CHECK (length(actor) BETWEEN 1 AND 120),
    completed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_commercial_readiness_latest
    ON commercial_readiness_evidence(evidence_type,git_commit,database_schema,completed_at DESC,evidence_id DESC);

CREATE OR REPLACE FUNCTION commercial_readiness_evidence_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'commercial_readiness_evidence is immutable';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_commercial_readiness_evidence_no_mutation
BEFORE UPDATE OR DELETE ON commercial_readiness_evidence
FOR EACH ROW EXECUTE FUNCTION commercial_readiness_evidence_immutable();
