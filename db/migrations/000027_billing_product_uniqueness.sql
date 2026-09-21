ALTER TABLE billing_subscriptions
    ADD COLUMN product_code TEXT;

UPDATE billing_subscriptions s
SET product_code=p.product_code
FROM billing_plans p
WHERE p.plan_id=s.plan_id AND s.product_code IS NULL;

ALTER TABLE billing_subscriptions
    ALTER COLUMN product_code SET NOT NULL;

ALTER TABLE billing_subscriptions
    ADD CONSTRAINT chk_billing_subscription_product
    CHECK (product_code IN ('WEBMASTER_PRO','AGENCY','SITE_SEARCH_PRO','BUSINESS_PRO','SEARCH_API','GEO_API'));

DROP INDEX IF EXISTS uq_billing_subscription_active_product;
CREATE UNIQUE INDEX uq_billing_subscription_active_product
    ON billing_subscriptions(account_id,product_code)
    WHERE status IN ('PENDING','ACTIVE','GRACE','PAST_DUE');
