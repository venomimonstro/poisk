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
