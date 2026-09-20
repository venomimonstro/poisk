ALTER TABLE address_staging_rows
    DROP CONSTRAINT IF EXISTS address_staging_rows_state_check;

ALTER TABLE address_staging_rows
    ADD CONSTRAINT address_staging_rows_state_check
    CHECK (state IN ('VALID','APPLIED','ORPHAN','IGNORED'));

CREATE INDEX idx_address_staging_ignored
    ON address_staging_rows(batch_id,record_kind,region_code,object_id)
    WHERE state='IGNORED';
