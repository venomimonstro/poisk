ALTER TABLE index_outbox
    DROP CONSTRAINT index_outbox_entity_type_entity_id_entity_version_operation_key;

ALTER TABLE index_outbox
    ADD CONSTRAINT uq_index_outbox_entity_version
    UNIQUE(entity_type, entity_id, entity_version);
