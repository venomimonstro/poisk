CREATE TABLE webmaster_users (
    user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (lower(email))
);

CREATE TABLE webmaster_sessions (
    session_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX idx_webmaster_sessions_user ON webmaster_sessions(user_id, expires_at DESC);
CREATE INDEX idx_webmaster_sessions_expiry ON webmaster_sessions(expires_at);

CREATE TABLE webmaster_sites (
    site_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE CASCADE,
    domain_id BIGINT NOT NULL REFERENCES domains(domain_id) ON DELETE RESTRICT,
    origin TEXT NOT NULL,
    host TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    verification_method TEXT CHECK (verification_method IS NULL OR verification_method IN ('DNS_TXT','HTML_FILE','META_TAG')),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','VERIFIED','SUSPENDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, host)
);
CREATE INDEX idx_webmaster_sites_user ON webmaster_sites(user_id, site_id);
CREATE INDEX idx_webmaster_sites_domain ON webmaster_sites(domain_id);

CREATE TABLE webmaster_verifications (
    verification_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    site_id BIGINT NOT NULL REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    token_hint TEXT NOT NULL,
    method TEXT NOT NULL CHECK (method IN ('DNS_TXT','HTML_FILE','META_TAG')),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','VERIFIED','EXPIRED','REVOKED')),
    expires_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE UNIQUE INDEX uq_webmaster_verification_active
    ON webmaster_verifications(site_id, method)
    WHERE status = 'PENDING';
CREATE INDEX idx_webmaster_verification_expiry ON webmaster_verifications(expires_at) WHERE status = 'PENDING';

CREATE TABLE webmaster_sitemaps (
    sitemap_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    site_id BIGINT NOT NULL REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    sitemap_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'SUBMITTED' CHECK (status IN ('SUBMITTED','QUEUED','FETCHED','FAILED','DELETED')),
    last_error TEXT,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(site_id, sitemap_url)
);

CREATE TABLE webmaster_url_requests (
    request_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    site_id BIGINT NOT NULL REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    url_id BIGINT REFERENCES urls(url_id) ON DELETE SET NULL,
    normalized_url TEXT NOT NULL,
    operation TEXT NOT NULL CHECK (operation IN ('SUBMIT','REINDEX','DELETE')),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','QUEUED','DONE','REJECTED','FAILED')),
    diagnostic TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webmaster_url_requests_site ON webmaster_url_requests(site_id, created_at DESC);
CREATE UNIQUE INDEX uq_webmaster_url_request_active
    ON webmaster_url_requests(site_id, normalized_url, operation)
    WHERE status IN ('PENDING','QUEUED');

CREATE TABLE webmaster_metrics_daily (
    site_id BIGINT NOT NULL REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    day DATE NOT NULL,
    impressions BIGINT NOT NULL DEFAULT 0 CHECK (impressions >= 0),
    clicks BIGINT NOT NULL DEFAULT 0 CHECK (clicks >= 0),
    answer_citations BIGINT NOT NULL DEFAULT 0 CHECK (answer_citations >= 0),
    PRIMARY KEY(site_id, day),
    CHECK (clicks <= impressions OR impressions = 0)
);
