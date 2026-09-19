CREATE TABLE document_content (
    url_id BIGINT NOT NULL,
    version BIGINT NOT NULL CHECK (version > 0),
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    lang TEXT NOT NULL DEFAULT '',
    canonical_url TEXT,
    clean_text TEXT NOT NULL DEFAULT '',
    robots_noindex BOOLEAN NOT NULL DEFAULT FALSE,
    robots_nofollow BOOLEAN NOT NULL DEFAULT FALSE,
    structured_data JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_hash BYTEA NOT NULL,
    simhash BIGINT NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (url_id, version),
    CONSTRAINT fk_document_content_version
        FOREIGN KEY (url_id, version)
        REFERENCES document_versions(url_id, version)
        ON DELETE CASCADE
);

CREATE INDEX idx_document_content_rebuild
    ON document_content(url_id, version DESC)
    WHERE robots_noindex = FALSE;

CREATE INDEX idx_document_content_hash
    ON document_content(content_hash);
