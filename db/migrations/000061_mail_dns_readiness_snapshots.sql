CREATE TABLE mail_dns_readiness_snapshots (
    snapshot_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain TEXT NOT NULL CHECK (length(domain) BETWEEN 3 AND 253),
    selector TEXT NOT NULL CHECK (length(selector) BETWEEN 1 AND 63),
    mx_ok BOOLEAN NOT NULL,
    spf_ok BOOLEAN NOT NULL,
    dmarc_ok BOOLEAN NOT NULL,
    dkim_ok BOOLEAN NOT NULL,
    ready BOOLEAN NOT NULL,
    drift BOOLEAN NOT NULL DEFAULT FALSE,
    fingerprint CHAR(64) NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    reasons JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(reasons)='array' AND jsonb_array_length(reasons) <= 8),
    checked_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (domain = lower(domain)),
    CHECK (selector = lower(selector)),
    CHECK (ready = (mx_ok AND spf_ok AND dmarc_ok AND dkim_ok))
);
CREATE INDEX idx_mail_dns_readiness_latest ON mail_dns_readiness_snapshots(domain,selector,checked_at DESC,snapshot_id DESC);
CREATE INDEX idx_mail_dns_readiness_drift ON mail_dns_readiness_snapshots(checked_at DESC,snapshot_id DESC) WHERE drift;

CREATE OR REPLACE FUNCTION mail_dns_readiness_snapshots_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'mail_dns_readiness_snapshots is immutable';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_mail_dns_readiness_snapshots_no_update
BEFORE UPDATE OR DELETE ON mail_dns_readiness_snapshots
FOR EACH ROW EXECUTE FUNCTION mail_dns_readiness_snapshots_immutable();
