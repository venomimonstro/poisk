ALTER TABLE datahub_pages
    ADD COLUMN manual_suppressed BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_datahub_manual_suppressed
    ON datahub_pages(page_id) WHERE manual_suppressed=TRUE;

CREATE OR REPLACE FUNCTION datahub_enforce_manual_suppression()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.manual_suppressed=TRUE AND NEW.manual_suppressed=TRUE THEN
        NEW.state := 'SUPPRESSED';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_datahub_manual_suppression
BEFORE UPDATE ON datahub_pages
FOR EACH ROW EXECUTE FUNCTION datahub_enforce_manual_suppression();
