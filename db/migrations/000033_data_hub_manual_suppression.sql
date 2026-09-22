ALTER TABLE datahub_pages
    ADD COLUMN manual_suppressed BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_datahub_manual_suppressed
    ON datahub_pages(page_id) WHERE manual_suppressed=TRUE;
