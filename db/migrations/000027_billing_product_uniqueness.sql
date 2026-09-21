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

WITH ranked AS (
    SELECT subscription_id,
           row_number() OVER (
               PARTITION BY account_id,product_code
               ORDER BY CASE status
                   WHEN 'ACTIVE' THEN 1
                   WHEN 'GRACE' THEN 2
                   WHEN 'PAST_DUE' THEN 3
                   WHEN 'PENDING' THEN 4
                   ELSE 5
               END,
               subscription_id DESC
           ) AS rn
    FROM billing_subscriptions
    WHERE status IN ('PENDING','ACTIVE','GRACE','PAST_DUE')
)
UPDATE billing_subscriptions s
SET status='CANCELED',canceled_at=COALESCE(canceled_at,now()),updated_at=now()
FROM ranked r
WHERE s.subscription_id=r.subscription_id AND r.rn>1;

DROP INDEX IF EXISTS uq_billing_subscription_active_product;
CREATE UNIQUE INDEX uq_billing_subscription_active_product
    ON billing_subscriptions(account_id,product_code)
    WHERE status IN ('PENDING','ACTIVE','GRACE','PAST_DUE');
