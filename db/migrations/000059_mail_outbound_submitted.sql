ALTER TABLE mail_outbound_deliveries DROP CONSTRAINT IF EXISTS mail_outbound_deliveries_status_check;
ALTER TABLE mail_outbound_deliveries
    ADD CONSTRAINT mail_outbound_deliveries_status_check
    CHECK (status IN ('READY','LEASED','RETRY','SUBMITTED','DELIVERED','BOUNCED','DEAD'));

ALTER TABLE mail_outbound_deliveries ADD COLUMN submitted_at TIMESTAMPTZ;
ALTER TABLE mail_outbound_deliveries
    ADD CONSTRAINT mail_outbound_submitted_state CHECK ((status='SUBMITTED' AND submitted_at IS NOT NULL) OR status<>'SUBMITTED');
CREATE INDEX idx_mail_outbound_submitted ON mail_outbound_deliveries(submitted_at,delivery_id) WHERE status='SUBMITTED';

ALTER TABLE mail_outbound_events DROP CONSTRAINT IF EXISTS mail_outbound_events_action_check;
ALTER TABLE mail_outbound_events
    ADD CONSTRAINT mail_outbound_events_action_check
    CHECK (action IN ('QUEUE','LEASE','RETRY','SUBMIT','DELIVER','BOUNCE','DEAD','LEASE_EXPIRE'));

ALTER TABLE mail_gateway_events ADD COLUMN source_event_id TEXT;
ALTER TABLE mail_gateway_events
    ADD CONSTRAINT mail_gateway_source_event_length CHECK (source_event_id IS NULL OR length(source_event_id) BETWEEN 16 AND 160);
CREATE UNIQUE INDEX uq_mail_gateway_source_event ON mail_gateway_events(source_event_id) WHERE source_event_id IS NOT NULL;
