CREATE TABLE datahub_build_state (
    job_key TEXT PRIMARY KEY CHECK (job_key IN ('DIRECTORIES','ORGANIZATIONS','WEBSITES')),
    cursor_text TEXT NOT NULL DEFAULT '',
    last_run_at TIMESTAMPTZ,
    last_completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(cursor_text) <= 256)
);

INSERT INTO datahub_build_state(job_key) VALUES
('DIRECTORIES'),('ORGANIZATIONS'),('WEBSITES')
ON CONFLICT(job_key) DO NOTHING;
