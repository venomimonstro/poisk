ALTER TABLE mail_dns_readiness_snapshots
    ADD COLUMN IF NOT EXISTS drift BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS fingerprint CHAR(64);

UPDATE mail_dns_readiness_snapshots
SET fingerprint = encode(digest(
    concat_ws('|',mx_ok::text,spf_ok::text,dmarc_ok::text,dkim_ok::text,ready::text,array_to_string(reasons,',')),
    'sha256'
),'hex')
WHERE fingerprint IS NULL;

ALTER TABLE mail_dns_readiness_snapshots
    ALTER COLUMN fingerprint SET NOT NULL;

ALTER TABLE mail_dns_readiness_snapshots
    DROP CONSTRAINT IF EXISTS mail_dns_readiness_snapshots_fingerprint_check;
ALTER TABLE mail_dns_readiness_snapshots
    ADD CONSTRAINT mail_dns_readiness_snapshots_fingerprint_check
    CHECK (fingerprint ~ '^[0-9a-f]{64}$');

CREATE INDEX IF NOT EXISTS idx_mail_dns_readiness_latest
    ON mail_dns_readiness_snapshots(domain,selector,checked_at DESC,snapshot_id DESC);
CREATE INDEX IF NOT EXISTS idx_mail_dns_readiness_drift
    ON mail_dns_readiness_snapshots(checked_at DESC,snapshot_id DESC) WHERE drift;

CREATE OR REPLACE FUNCTION mail_dns_readiness_snapshots_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'mail_dns_readiness_snapshots is immutable';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_mail_dns_readiness_snapshots_no_update ON mail_dns_readiness_snapshots;
CREATE TRIGGER trg_mail_dns_readiness_snapshots_no_update
BEFORE UPDATE OR DELETE ON mail_dns_readiness_snapshots
FOR EACH ROW EXECUTE FUNCTION mail_dns_readiness_snapshots_immutable();
