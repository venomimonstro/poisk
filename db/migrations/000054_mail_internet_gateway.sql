CREATE TABLE mail_external_aliases (
    alias_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    local_part TEXT NOT NULL,
    domain TEXT NOT NULL,
    address TEXT GENERATED ALWAYS AS (lower(local_part) || '@' || lower(domain)) STORED,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (local_part ~ '^[a-z0-9][a-z0-9._+-]{0,63}$'),
    CHECK (domain ~ '^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$'),
    UNIQUE(local_part,domain)
);
CREATE UNIQUE INDEX uq_mail_external_alias_primary ON mail_external_aliases(mailbox_id) WHERE is_primary AND status='ACTIVE';
CREATE INDEX idx_mail_external_alias_mailbox ON mail_external_aliases(mailbox_id,status,alias_id);

CREATE TABLE mail_external_recipients (
    external_recipient_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    address TEXT NOT NULL,
    recipient_type TEXT NOT NULL CHECK (recipient_type IN ('TO','CC','BCC')),
    ordinal SMALLINT NOT NULL CHECK (ordinal BETWEEN 1 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(address) BETWEEN 3 AND 320),
    CHECK (position('@' in address) > 1),
    UNIQUE(message_id,address,recipient_type),
    UNIQUE(message_id,recipient_type,ordinal)
);
CREATE INDEX idx_mail_external_recipients_message ON mail_external_recipients(message_id,recipient_type,ordinal);

CREATE TABLE mail_outbound_deliveries (
    delivery_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    external_recipient_id BIGINT NOT NULL UNIQUE REFERENCES mail_external_recipients(external_recipient_id) ON DELETE RESTRICT,
    sender_mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'READY' CHECK (status IN ('READY','LEASED','RETRY','DELIVERED','BOUNCED','DEAD')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 100),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_owner TEXT,
    lease_until TIMESTAMPTZ,
    idempotency_key CHAR(64) NOT NULL UNIQUE CHECK (idempotency_key ~ '^[0-9a-f]{64}$'),
    remote_queue_id TEXT CHECK (remote_queue_id IS NULL OR length(remote_queue_id) <= 255),
    last_error_code TEXT CHECK (last_error_code IS NULL OR length(last_error_code) <= 80),
    last_error_detail TEXT CHECK (last_error_detail IS NULL OR length(last_error_detail) <= 1000),
    delivered_at TIMESTAMPTZ,
    bounced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status='LEASED' AND lease_owner IS NOT NULL AND lease_until IS NOT NULL) OR status<>'LEASED'),
    CHECK ((status='DELIVERED' AND delivered_at IS NOT NULL) OR status<>'DELIVERED'),
    CHECK ((status='BOUNCED' AND bounced_at IS NOT NULL) OR status<>'BOUNCED')
);
CREATE INDEX idx_mail_outbound_ready ON mail_outbound_deliveries(status,next_attempt_at,delivery_id) WHERE status IN ('READY','RETRY');
CREATE INDEX idx_mail_outbound_lease ON mail_outbound_deliveries(lease_until) WHERE status='LEASED';
CREATE INDEX idx_mail_outbound_sender ON mail_outbound_deliveries(sender_mailbox_id,created_at DESC,delivery_id DESC);

CREATE TABLE mail_outbound_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    delivery_id BIGINT NOT NULL REFERENCES mail_outbound_deliveries(delivery_id) ON DELETE RESTRICT,
    action TEXT NOT NULL CHECK (action IN ('QUEUE','LEASE','RETRY','DELIVER','BOUNCE','DEAD','LEASE_EXPIRE')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt BETWEEN 0 AND 100),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_mail_outbound_events_delivery ON mail_outbound_events(delivery_id,event_id DESC);

CREATE TABLE mail_gateway_replay_guard (
    event_id TEXT PRIMARY KEY CHECK (length(event_id) BETWEEN 16 AND 160),
    body_sha256 CHAR(64) NOT NULL CHECK (body_sha256 ~ '^[0-9a-f]{64}$'),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (expires_at > received_at)
);
CREATE INDEX idx_mail_gateway_replay_expiry ON mail_gateway_replay_guard(expires_at);

CREATE TABLE mail_gateway_events (
    gateway_event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    direction TEXT NOT NULL CHECK (direction IN ('INBOUND','OUTBOUND')),
    event_type TEXT NOT NULL CHECK (event_type IN ('ACCEPT','REJECT','DELIVER','BOUNCE','DEFER','AUTH_FAIL')),
    message_id BIGINT REFERENCES mail_messages(message_id) ON DELETE RESTRICT,
    delivery_id BIGINT REFERENCES mail_outbound_deliveries(delivery_id) ON DELETE RESTRICT,
    mailbox_id BIGINT REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    code TEXT CHECK (code IS NULL OR length(code) <= 80),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_mail_gateway_events_created ON mail_gateway_events(created_at DESC,gateway_event_id DESC);
CREATE INDEX idx_mail_gateway_events_delivery ON mail_gateway_events(delivery_id,gateway_event_id DESC) WHERE delivery_id IS NOT NULL;
