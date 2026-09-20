CREATE TABLE site_search_widgets (
    widget_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    site_id BIGINT NOT NULL UNIQUE REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    public_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','REVOKED')),
    max_results SMALLINT NOT NULL DEFAULT 10 CHECK (max_results BETWEEN 1 AND 20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (public_key ~ '^psw_[A-Za-z0-9_-]{24,64}$')
);
CREATE INDEX idx_site_search_widgets_site_status ON site_search_widgets(site_id,status);

CREATE TABLE site_search_usage_daily (
    widget_id BIGINT NOT NULL REFERENCES site_search_widgets(widget_id) ON DELETE CASCADE,
    day DATE NOT NULL,
    requests BIGINT NOT NULL DEFAULT 0 CHECK (requests >= 0),
    results BIGINT NOT NULL DEFAULT 0 CHECK (results >= 0),
    zero_results BIGINT NOT NULL DEFAULT 0 CHECK (zero_results >= 0),
    PRIMARY KEY(widget_id,day)
);
