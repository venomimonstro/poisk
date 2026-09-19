CREATE TABLE domains (
    domain_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    host TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','PAUSED','DISABLED')),
    policy TEXT NOT NULL DEFAULT 'ALLOW' CHECK (policy IN ('ALLOW','LIMITED','BLOCK','REVIEW')),
    trust_level SMALLINT NOT NULL DEFAULT 0 CHECK (trust_level BETWEEN 0 AND 100),
    crawl_budget INTEGER NOT NULL DEFAULT 100 CHECK (crawl_budget >= 0),
    max_urls INTEGER NOT NULL DEFAULT 500 CHECK (max_urls >= 0),
    max_depth SMALLINT NOT NULL DEFAULT 3 CHECK (max_depth BETWEEN 0 AND 64),
    requests_per_second DOUBLE PRECISION NOT NULL DEFAULT 0.5 CHECK (requests_per_second > 0 AND requests_per_second <= 1000),
    max_concurrency SMALLINT NOT NULL DEFAULT 1 CHECK (max_concurrency BETWEEN 1 AND 1024),
    max_bytes_per_day BIGINT NOT NULL DEFAULT 104857600 CHECK (max_bytes_per_day >= 0),
    duplicate_ratio DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (duplicate_ratio BETWEEN 0 AND 1),
    error_ratio DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (error_ratio BETWEEN 0 AND 1),
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (quality_score BETWEEN 0 AND 100),
    demand_score DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (demand_score >= 0),
    last_crawl_at TIMESTAMPTZ,
    next_crawl_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_domains_schedule
    ON domains (next_crawl_at, demand_score DESC)
    WHERE status = 'ACTIVE' AND policy IN ('ALLOW','LIMITED');

CREATE TABLE urls (
    url_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    normalized_url TEXT NOT NULL UNIQUE,
    discovered_from_url_id BIGINT REFERENCES urls(url_id) ON DELETE SET NULL,
    canonical_url_id BIGINT REFERENCES urls(url_id) ON DELETE SET NULL,
    http_status SMALLINT CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    crawl_status TEXT NOT NULL DEFAULT 'DISCOVERED' CHECK (crawl_status IN ('DISCOVERED','QUEUED','FETCHED','FAILED','BLOCKED')),
    index_status TEXT NOT NULL DEFAULT 'NOT_INDEXED' CHECK (index_status IN ('NOT_INDEXED','INDEXED','EXCLUDED','DELETED','ERROR')),
    last_crawl_at TIMESTAMPTZ,
    next_crawl_at TIMESTAMPTZ,
    etag TEXT,
    last_modified TIMESTAMPTZ,
    content_hash BYTEA,
    simhash BIGINT,
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (quality_score BETWEEN 0 AND 100),
    spam_score DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (spam_score BETWEEN 0 AND 100),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (canonical_url_id IS NULL OR canonical_url_id <> url_id)
);

CREATE INDEX idx_urls_domain ON urls(domain_id, url_id);
CREATE INDEX idx_urls_recrawl ON urls(next_crawl_at) WHERE crawl_status <> 'BLOCKED';
CREATE INDEX idx_urls_content_hash ON urls(content_hash) WHERE content_hash IS NOT NULL;

CREATE TABLE document_versions (
    url_id BIGINT NOT NULL REFERENCES urls(url_id) ON DELETE CASCADE,
    version BIGINT NOT NULL CHECK (version > 0),
    http_status SMALLINT CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    content_hash BYTEA,
    simhash BIGINT,
    content_length BIGINT CHECK (content_length IS NULL OR content_length >= 0),
    content_type TEXT,
    extraction_status TEXT NOT NULL DEFAULT 'PENDING' CHECK (extraction_status IN ('PENDING','READY','FAILED','SKIPPED')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (url_id, version)
);

CREATE TABLE crawl_queue (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url_id BIGINT NOT NULL REFERENCES urls(url_id) ON DELETE CASCADE,
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    generation BIGINT NOT NULL DEFAULT 1 CHECK (generation > 0),
    priority DOUBLE PRECISION NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'READY' CHECK (status IN ('READY','LEASED','RETRY','DONE','DEAD')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    worker_id TEXT,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'LEASED' AND lease_until IS NOT NULL AND worker_id IS NOT NULL) OR status <> 'LEASED')
);

CREATE UNIQUE INDEX uq_crawl_queue_active_url_generation
    ON crawl_queue(url_id, generation)
    WHERE status IN ('READY','LEASED','RETRY');

CREATE INDEX idx_crawl_queue_schedule
    ON crawl_queue(priority DESC, available_at ASC, id ASC)
    WHERE status IN ('READY','RETRY');

CREATE INDEX idx_crawl_queue_expired_leases
    ON crawl_queue(lease_until)
    WHERE status = 'LEASED';

CREATE TABLE crawl_history (
    history_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id BIGINT,
    url_id BIGINT NOT NULL REFERENCES urls(url_id) ON DELETE CASCADE,
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE CASCADE,
    worker_id TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('SUCCESS','RETRY','DEAD','BLOCKED','NOT_MODIFIED')),
    http_status SMALLINT CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    bytes_received BIGINT CHECK (bytes_received IS NULL OR bytes_received >= 0),
    error_code TEXT,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_crawl_history_url_time ON crawl_history(url_id, completed_at DESC);
CREATE INDEX idx_crawl_history_domain_time ON crawl_history(domain_id, completed_at DESC);

CREATE TABLE index_outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('WEB_DOCUMENT','ORGANIZATION','ADDRESS')),
    entity_id BIGINT NOT NULL,
    entity_version BIGINT NOT NULL CHECK (entity_version > 0),
    operation TEXT NOT NULL CHECK (operation IN ('UPSERT','DELETE')),
    status TEXT NOT NULL DEFAULT 'READY' CHECK (status IN ('READY','LEASED','RETRY','PROCESSED','DEAD')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 10 CHECK (max_attempts > 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    worker_id TEXT,
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entity_type, entity_id, entity_version, operation),
    CHECK ((status = 'LEASED' AND lease_until IS NOT NULL AND worker_id IS NOT NULL) OR status <> 'LEASED'),
    CHECK ((status = 'PROCESSED' AND processed_at IS NOT NULL) OR status <> 'PROCESSED')
);

CREATE INDEX idx_index_outbox_schedule
    ON index_outbox(available_at ASC, id ASC)
    WHERE status IN ('READY','RETRY');

CREATE INDEX idx_index_outbox_expired_leases
    ON index_outbox(lease_until)
    WHERE status = 'LEASED';

CREATE TABLE audit_log (
    audit_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('SYSTEM','USER','ADMIN','WORKER')),
    actor_id TEXT,
    action TEXT NOT NULL,
    entity_type TEXT,
    entity_id TEXT,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id, created_at DESC);
CREATE INDEX idx_audit_log_time ON audit_log(created_at DESC);

CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
