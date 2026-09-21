CREATE TABLE billing_plans (
    plan_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_code TEXT NOT NULL CHECK (product_code IN ('WEBMASTER_PRO','AGENCY','SITE_SEARCH_PRO','BUSINESS_PRO','SEARCH_API','GEO_API')),
    plan_code TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    monthly_price_kopecks BIGINT NOT NULL CHECK (monthly_price_kopecks >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
    quotas JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','RETIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(plan_code,version),
    CHECK (plan_code ~ '^[A-Z0-9_]{3,64}$'),
    CHECK (jsonb_typeof(quotas)='object')
);
CREATE INDEX idx_billing_plans_product_active ON billing_plans(product_code,status,plan_id);

CREATE TABLE billing_accounts (
    account_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    webmaster_user_id BIGINT REFERENCES webmaster_users(user_id) ON DELETE RESTRICT,
    agency_id BIGINT REFERENCES agencies(agency_id) ON DELETE RESTRICT,
    place_id BIGINT REFERENCES organizations(place_id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED','CLOSED')),
    currency CHAR(3) NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(webmaster_user_id,agency_id,place_id)=1)
);
CREATE UNIQUE INDEX uq_billing_account_webmaster ON billing_accounts(webmaster_user_id) WHERE webmaster_user_id IS NOT NULL;
CREATE UNIQUE INDEX uq_billing_account_agency ON billing_accounts(agency_id) WHERE agency_id IS NOT NULL;
CREATE UNIQUE INDEX uq_billing_account_place ON billing_accounts(place_id) WHERE place_id IS NOT NULL;

CREATE TABLE billing_subscriptions (
    subscription_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES billing_accounts(account_id) ON DELETE RESTRICT,
    plan_id BIGINT NOT NULL REFERENCES billing_plans(plan_id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('PENDING','ACTIVE','GRACE','PAST_DUE','CANCELED','EXPIRED')),
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    grace_until TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    activated_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (current_period_end > current_period_start),
    CHECK (grace_until IS NULL OR grace_until >= current_period_end)
);
CREATE UNIQUE INDEX uq_billing_subscription_active_product
    ON billing_subscriptions(account_id,plan_id)
    WHERE status IN ('PENDING','ACTIVE','GRACE','PAST_DUE');
CREATE INDEX idx_billing_subscriptions_account ON billing_subscriptions(account_id,status,current_period_end DESC);
CREATE INDEX idx_billing_subscriptions_expiry ON billing_subscriptions(current_period_end) WHERE status IN ('ACTIVE','GRACE','PAST_DUE');

CREATE TABLE billing_invoices (
    invoice_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES billing_accounts(account_id) ON DELETE RESTRICT,
    subscription_id BIGINT REFERENCES billing_subscriptions(subscription_id) ON DELETE SET NULL,
    external_reference TEXT UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('OPEN','PAID','VOID','FAILED','REFUNDED')),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    due_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (period_end IS NULL OR period_start IS NOT NULL),
    CHECK (period_end IS NULL OR period_end > period_start)
);
CREATE INDEX idx_billing_invoices_account ON billing_invoices(account_id,created_at DESC);

CREATE TABLE billing_payment_events (
    payment_event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider ~ '^[A-Z0-9_]{2,32}$'),
    provider_event_id TEXT NOT NULL CHECK (length(provider_event_id) BETWEEN 1 AND 160),
    event_type TEXT NOT NULL CHECK (event_type IN ('PAYMENT_SUCCEEDED','PAYMENT_FAILED','REFUND_SUCCEEDED','CHARGEBACK')),
    invoice_id BIGINT REFERENCES billing_invoices(invoice_id) ON DELETE RESTRICT,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash)=32),
    occurred_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider,provider_event_id)
);
CREATE INDEX idx_billing_payment_events_unprocessed ON billing_payment_events(payment_event_id) WHERE processed_at IS NULL;

CREATE TABLE billing_ledger_entries (
    ledger_entry_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES billing_accounts(account_id) ON DELETE RESTRICT,
    invoice_id BIGINT REFERENCES billing_invoices(invoice_id) ON DELETE RESTRICT,
    payment_event_id BIGINT REFERENCES billing_payment_events(payment_event_id) ON DELETE RESTRICT,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('INVOICE','PAYMENT','REFUND','CREDIT','ADJUSTMENT')),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks <> 0),
    currency CHAR(3) NOT NULL DEFAULT 'RUB' CHECK (currency='RUB'),
    idempotency_key TEXT NOT NULL UNIQUE CHECK (length(idempotency_key) BETWEEN 8 AND 200),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (jsonb_typeof(details)='object')
);
CREATE INDEX idx_billing_ledger_account ON billing_ledger_entries(account_id,ledger_entry_id);

CREATE TABLE billing_usage_periods (
    usage_period_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES billing_accounts(account_id) ON DELETE CASCADE,
    metric_key TEXT NOT NULL CHECK (metric_key ~ '^[a-z0-9_.-]{2,64}$'),
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    used BIGINT NOT NULL DEFAULT 0 CHECK (used >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id,metric_key,period_start,period_end),
    CHECK (period_end > period_start)
);
CREATE INDEX idx_billing_usage_active ON billing_usage_periods(account_id,metric_key,period_end DESC);

INSERT INTO billing_plans(product_code,plan_code,version,monthly_price_kopecks,quotas) VALUES
('WEBMASTER_PRO','WEBMASTER_PRO_1490',1,149000,'{"sites":25,"url_requests_month":5000,"sitemaps_month":500}'::jsonb),
('AGENCY','AGENCY_4990',1,499000,'{"members":5,"sites":25,"managed_requests_month":10000}'::jsonb),
('AGENCY','AGENCY_9990',1,999000,'{"members":20,"sites":100,"managed_requests_month":50000}'::jsonb),
('SITE_SEARCH_PRO','SITE_SEARCH_PRO_1490',1,149000,'{"requests_month":50000,"max_results":20}'::jsonb),
('SITE_SEARCH_PRO','SITE_SEARCH_PRO_4990',1,499000,'{"requests_month":300000,"max_results":20}'::jsonb),
('BUSINESS_PRO','BUSINESS_PRO_990',1,99000,'{"claimed_places":1}'::jsonb),
('BUSINESS_PRO','BUSINESS_PRO_1990',1,199000,'{"claimed_places":5}'::jsonb)
ON CONFLICT(plan_code,version) DO NOTHING;
