ALTER TABLE index_outbox
    DROP CONSTRAINT index_outbox_status_check;

ALTER TABLE index_outbox
    ADD CONSTRAINT index_outbox_status_check
    CHECK (status IN ('READY','LEASED','RETRY','PROCESSED','SUPERSEDED','DEAD'));

CREATE INDEX idx_index_outbox_entity_version
    ON index_outbox(entity_type, entity_id, entity_version DESC, id DESC);
