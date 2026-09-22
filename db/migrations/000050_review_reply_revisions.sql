CREATE TABLE organization_review_reply_revisions (
    revision_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reply_id BIGINT NOT NULL REFERENCES organization_review_replies(reply_id) ON DELETE RESTRICT,
    version BIGINT NOT NULL CHECK (version > 0),
    body TEXT NOT NULL CHECK (length(body) BETWEEN 2 AND 3000),
    status TEXT NOT NULL CHECK (status IN ('VISIBLE','DELETED')),
    claim_id BIGINT NOT NULL REFERENCES organization_claims(claim_id) ON DELETE RESTRICT,
    owner_consumer_user_id BIGINT NOT NULL REFERENCES consumer_users(user_id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(reply_id,version)
);
CREATE INDEX idx_org_review_reply_revisions_reply ON organization_review_reply_revisions(reply_id,version DESC);

CREATE OR REPLACE FUNCTION organization_review_reply_snapshot_after() RETURNS trigger AS $$
BEGIN
    INSERT INTO organization_review_reply_revisions(reply_id,version,body,status,claim_id,owner_consumer_user_id)
    VALUES(NEW.reply_id,NEW.version,NEW.body,NEW.status,NEW.claim_id,NEW.owner_consumer_user_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_reply_snapshot
AFTER INSERT OR UPDATE ON organization_review_replies
FOR EACH ROW EXECUTE FUNCTION organization_review_reply_snapshot_after();

INSERT INTO organization_review_reply_revisions(reply_id,version,body,status,claim_id,owner_consumer_user_id,created_at)
SELECT reply_id,version,body,status,claim_id,owner_consumer_user_id,updated_at
FROM organization_review_replies
ON CONFLICT(reply_id,version) DO NOTHING;

CREATE OR REPLACE FUNCTION organization_review_reply_revisions_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'organization_review_reply_revisions is immutable';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_reply_revisions_no_update
BEFORE UPDATE OR DELETE ON organization_review_reply_revisions
FOR EACH ROW EXECUTE FUNCTION organization_review_reply_revisions_immutable();

CREATE OR REPLACE FUNCTION organization_review_replies_no_delete() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'organization_review_replies must be soft-deleted';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_replies_no_delete
BEFORE DELETE ON organization_review_replies
FOR EACH ROW EXECUTE FUNCTION organization_review_replies_no_delete();
