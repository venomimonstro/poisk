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
