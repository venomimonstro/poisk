CREATE TABLE capacity_benchmark_runs (
    run_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    label TEXT NOT NULL CHECK (length(label) BETWEEN 1 AND 128),
    mode TEXT NOT NULL CHECK (mode IN ('LIVE_READONLY','ISOLATED_1M')),
    status TEXT NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING','COMPLETED','FAILED','CANCELLED')),
    target_documents BIGINT NOT NULL CHECK (target_documents > 0 AND target_documents <= 100000000),
    config JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config)='object'),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    last_error TEXT,
    CHECK ((status='RUNNING' AND completed_at IS NULL) OR (status<>'RUNNING' AND completed_at IS NOT NULL))
);

CREATE TABLE capacity_snapshots (
    snapshot_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id BIGINT NOT NULL UNIQUE REFERENCES capacity_benchmark_runs(run_id) ON DELETE RESTRICT,
    measured_documents BIGINT NOT NULL CHECK (measured_documents >= 0),
    measured_at TIMESTAMPTZ NOT NULL,
    workload JSONB NOT NULL CHECK (jsonb_typeof(workload)='object'),
    resources JSONB NOT NULL CHECK (jsonb_typeof(resources)='object'),
    queues JSONB NOT NULL CHECK (jsonb_typeof(queues)='object'),
    storage JSONB NOT NULL CHECK (jsonb_typeof(storage)='object'),
    projection_10m JSONB NOT NULL CHECK (jsonb_typeof(projection_10m)='object'),
    bottlenecks JSONB NOT NULL CHECK (jsonb_typeof(bottlenecks)='array'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE capacity_adr_decisions (
    decision_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    snapshot_id BIGINT NOT NULL UNIQUE REFERENCES capacity_snapshots(snapshot_id) ON DELETE RESTRICT,
    choice TEXT NOT NULL CHECK (choice IN ('STAY_SINGLE_NODE','MOVE_CRAWLER','SHARD_SEARCH','ADD_REPLICA')),
    rationale TEXT NOT NULL CHECK (length(rationale) BETWEEN 20 AND 8000),
    decided_by TEXT NOT NULL CHECK (length(decided_by) BETWEEN 2 AND 128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION reject_capacity_snapshot_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'capacity snapshots and ADR decisions are immutable';
END;
$$;

CREATE TRIGGER trg_capacity_snapshots_immutable
BEFORE UPDATE OR DELETE ON capacity_snapshots
FOR EACH ROW EXECUTE FUNCTION reject_capacity_snapshot_mutation();

CREATE TRIGGER trg_capacity_adr_immutable
BEFORE UPDATE OR DELETE ON capacity_adr_decisions
FOR EACH ROW EXECUTE FUNCTION reject_capacity_snapshot_mutation();

CREATE INDEX idx_capacity_runs_started ON capacity_benchmark_runs(started_at DESC,run_id DESC);
