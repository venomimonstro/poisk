-- Repair historical duplicate migration version 000050.
-- Existing installations may have applied either branch under numeric version 50.
-- This migration idempotently converges both branches to the same schema/state.

-- Branch A: owner self-review suppression after a verified organization claim.
CREATE OR REPLACE FUNCTION suppress_owner_self_review_on_claim() RETURNS trigger AS $$
DECLARE
    consumer_id BIGINT;
BEGIN
    IF NEW.status <> 'ACTIVE' THEN
        RETURN NEW;
    END IF;

    SELECT consumer_user_id INTO consumer_id
    FROM webmaster_users
    WHERE user_id = NEW.user_id;

    IF consumer_id IS NULL THEN
        RETURN NEW;
    END IF;

    UPDATE organization_reviews
       SET status='HIDDEN',
           change_actor_type='SYSTEM',
           change_actor_id=NULL,
           change_reason='OWNER_CLAIM_SELF_REVIEW'
     WHERE place_id=NEW.place_id
       AND consumer_user_id=consumer_id
       AND status IN ('VISIBLE','PENDING');

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_suppress_owner_self_review_on_claim ON organization_claims;
CREATE TRIGGER trg_suppress_owner_self_review_on_claim
AFTER INSERT OR UPDATE OF status ON organization_claims
FOR EACH ROW
WHEN (NEW.status='ACTIVE')
EXECUTE FUNCTION suppress_owner_self_review_on_claim();

-- Also converge already-active claims created before the trigger existed.
UPDATE organization_reviews r
SET status='HIDDEN',
    change_actor_type='SYSTEM',
    change_actor_id=NULL,
    change_reason='OWNER_CLAIM_SELF_REVIEW'
FROM organization_claims c
JOIN webmaster_users w ON w.user_id=c.user_id
WHERE c.status='ACTIVE'
  AND r.place_id=c.place_id
  AND r.consumer_user_id=w.consumer_user_id
  AND r.status IN ('VISIBLE','PENDING');

-- Branch B: immutable owner-reply revisions.
CREATE TABLE IF NOT EXISTS organization_review_reply_revisions (
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
CREATE INDEX IF NOT EXISTS idx_org_review_reply_revisions_reply
    ON organization_review_reply_revisions(reply_id,version DESC);

CREATE OR REPLACE FUNCTION organization_review_reply_snapshot_after() RETURNS trigger AS $$
BEGIN
    INSERT INTO organization_review_reply_revisions(reply_id,version,body,status,claim_id,owner_consumer_user_id)
    VALUES(NEW.reply_id,NEW.version,NEW.body,NEW.status,NEW.claim_id,NEW.owner_consumer_user_id)
    ON CONFLICT(reply_id,version) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_org_review_reply_snapshot ON organization_review_replies;
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

DROP TRIGGER IF EXISTS trg_org_review_reply_revisions_no_update ON organization_review_reply_revisions;
CREATE TRIGGER trg_org_review_reply_revisions_no_update
BEFORE UPDATE OR DELETE ON organization_review_reply_revisions
FOR EACH ROW EXECUTE FUNCTION organization_review_reply_revisions_immutable();

CREATE OR REPLACE FUNCTION organization_review_replies_no_delete() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'organization_review_replies must be soft-deleted';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_org_review_replies_no_delete ON organization_review_replies;
CREATE TRIGGER trg_org_review_replies_no_delete
BEFORE DELETE ON organization_review_replies
FOR EACH ROW EXECUTE FUNCTION organization_review_replies_no_delete();
