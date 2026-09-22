CREATE TABLE quality_runs (
    run_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    queries INTEGER NOT NULL CHECK (queries >= 0),
    ndcg_10 DOUBLE PRECISION NOT NULL CHECK (ndcg_10 BETWEEN 0 AND 1),
    mrr DOUBLE PRECISION NOT NULL CHECK (mrr BETWEEN 0 AND 1),
    recall_10 DOUBLE PRECISION NOT NULL CHECK (recall_10 BETWEEN 0 AND 1),
    zero_result_rate DOUBLE PRECISION NOT NULL CHECK (zero_result_rate BETWEEN 0 AND 1),
    duplicate_10 DOUBLE PRECISION NOT NULL CHECK (duplicate_10 BETWEEN 0 AND 1),
    gate_pass BOOLEAN NOT NULL,
    gate_failures JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(gate_failures)='array'),
    thresholds JSONB NOT NULL CHECK (jsonb_typeof(thresholds)='object'),
    report JSONB NOT NULL CHECK (jsonb_typeof(report)='object'),
    source TEXT NOT NULL DEFAULT 'quality' CHECK (length(source) BETWEEN 1 AND 80),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_quality_runs_latest ON quality_runs(completed_at DESC,run_id DESC);

CREATE OR REPLACE FUNCTION quality_runs_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'quality_runs is immutable';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_quality_runs_no_update BEFORE UPDATE OR DELETE ON quality_runs FOR EACH ROW EXECUTE FUNCTION quality_runs_immutable();
