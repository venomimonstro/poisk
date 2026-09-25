CREATE TABLE mail_delivery_suppressions (
    suppression_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sender_mailbox_id BIGINT NOT NULL REFERENCES mailboxes(mailbox_id) ON DELETE RESTRICT,
    address_sha256 CHAR(64) NOT NULL CHECK (address_sha256 ~ '^[0-9a-f]{64}$'),
    reason TEXT NOT NULL CHECK (reason IN ('HARD_BOUNCE','MANUAL')),
    hard_bounce_count SMALLINT NOT NULL DEFAULT 0 CHECK (hard_bounce_count BETWEEN 0 AND 100),
    first_bounced_at TIMESTAMPTZ,
    last_bounced_at TIMESTAMPTZ,
    suppressed_until TIMESTAMPTZ,
    cleared_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(sender_mailbox_id,address_sha256),
    CHECK ((hard_bounce_count=0 AND first_bounced_at IS NULL AND last_bounced_at IS NULL) OR
           (hard_bounce_count>0 AND first_bounced_at IS NOT NULL AND last_bounced_at IS NOT NULL)),
    CHECK (suppressed_until IS NULL OR suppressed_until > first_bounced_at)
);
CREATE INDEX idx_mail_delivery_suppression_active
    ON mail_delivery_suppressions(sender_mailbox_id,suppressed_until)
    WHERE cleared_at IS NULL;

CREATE TABLE mail_domain_delivery_pressure (
    domain TEXT PRIMARY KEY,
    transient_failures INTEGER NOT NULL DEFAULT 0 CHECK (transient_failures BETWEEN 0 AND 1000000),
    hard_failures INTEGER NOT NULL DEFAULT 0 CHECK (hard_failures BETWEEN 0 AND 1000000),
    successes INTEGER NOT NULL DEFAULT 0 CHECK (successes BETWEEN 0 AND 1000000),
    window_started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cooldown_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(domain) BETWEEN 1 AND 253)
);
CREATE INDEX idx_mail_domain_delivery_cooldown
    ON mail_domain_delivery_pressure(cooldown_until)
    WHERE cooldown_until IS NOT NULL;

CREATE TABLE mail_dns_readiness_snapshots (
    snapshot_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain TEXT NOT NULL CHECK (length(domain) BETWEEN 1 AND 253),
    selector TEXT NOT NULL CHECK (length(selector) BETWEEN 1 AND 80),
    mx_ok BOOLEAN NOT NULL,
    spf_ok BOOLEAN NOT NULL,
    dmarc_ok BOOLEAN NOT NULL,
    dkim_ok BOOLEAN NOT NULL,
    ready BOOLEAN NOT NULL,
    reasons TEXT[] NOT NULL DEFAULT '{}',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (cardinality(reasons) <= 8)
);
CREATE INDEX idx_mail_dns_readiness_domain_time
    ON mail_dns_readiness_snapshots(domain,checked_at DESC,snapshot_id DESC);

ALTER TABLE mail_outbound_events DROP CONSTRAINT IF EXISTS mail_outbound_events_action_check;
ALTER TABLE mail_outbound_events
    ADD CONSTRAINT mail_outbound_events_action_check
    CHECK (action IN ('QUEUE','LEASE','RETRY','SUBMIT','DELIVER','BOUNCE','DEAD','LEASE_EXPIRE','SUPPRESS','DEAD_RETRY','COOLDOWN'));
