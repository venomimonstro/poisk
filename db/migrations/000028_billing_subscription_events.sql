CREATE TABLE billing_subscription_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES billing_subscriptions(subscription_id) ON DELETE RESTRICT,
    from_status TEXT,
    to_status TEXT NOT NULL CHECK (to_status IN ('PENDING','ACTIVE','GRACE','PAST_DUE','CANCELED','EXPIRED')),
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 2 AND 64),
    payment_event_id BIGINT REFERENCES billing_payment_events(payment_event_id) ON DELETE RESTRICT,
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (from_status IS NULL OR from_status IN ('PENDING','ACTIVE','GRACE','PAST_DUE','CANCELED','EXPIRED'))
);
CREATE INDEX idx_billing_subscription_events_subscription
    ON billing_subscription_events(subscription_id,event_id);

CREATE OR REPLACE FUNCTION billing_enforce_business_pro_claim()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    owner_user_id BIGINT;
    has_claim BOOLEAN;
BEGIN
    IF NEW.product_code <> 'BUSINESS_PRO' THEN
        RETURN NEW;
    END IF;

    SELECT webmaster_user_id
    INTO owner_user_id
    FROM billing_accounts
    WHERE account_id=NEW.account_id AND status='ACTIVE';

    IF owner_user_id IS NULL THEN
        RAISE EXCEPTION 'BUSINESS_PRO requires an active Webmaster user billing account'
            USING ERRCODE='check_violation';
    END IF;

    SELECT EXISTS(
        SELECT 1
        FROM organization_claims c
        JOIN organizations o ON o.place_id=c.place_id
        WHERE c.user_id=owner_user_id
          AND c.status='ACTIVE'
          AND o.status IN ('ACTIVE','REVIEW')
    ) INTO has_claim;

    IF NOT has_claim THEN
        RAISE EXCEPTION 'BUSINESS_PRO requires an active verified organization claim'
            USING ERRCODE='check_violation';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_billing_business_pro_claim
BEFORE INSERT OR UPDATE OF account_id,product_code ON billing_subscriptions
FOR EACH ROW EXECUTE FUNCTION billing_enforce_business_pro_claim();

CREATE OR REPLACE FUNCTION billing_audit_subscription_status()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP='INSERT' THEN
        INSERT INTO billing_subscription_events(subscription_id,from_status,to_status,reason)
        VALUES(NEW.subscription_id,NULL,NEW.status,'SUBSCRIPTION_CREATED');
    ELSIF NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO billing_subscription_events(subscription_id,from_status,to_status,reason)
        VALUES(NEW.subscription_id,OLD.status,NEW.status,'STATUS_CHANGED');
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_billing_subscription_status_audit
AFTER INSERT OR UPDATE OF status ON billing_subscriptions
FOR EACH ROW EXECUTE FUNCTION billing_audit_subscription_status();
