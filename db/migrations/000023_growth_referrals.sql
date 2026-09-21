CREATE TABLE growth_referrals (
    referral_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE CASCADE,
    code TEXT NOT NULL UNIQUE CHECK (code ~ '^ref_[A-Za-z0-9_-]{16,64}$'),
    campaign_key TEXT CHECK (campaign_key IS NULL OR campaign_key ~ '^[A-Za-z0-9._-]{1,64}$'),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','REVOKED')),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_growth_referrals_owner ON growth_referrals(owner_user_id,status,referral_id);

CREATE TABLE growth_attribution_sessions (
    attribution_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
    referral_id BIGINT REFERENCES growth_referrals(referral_id) ON DELETE SET NULL,
    campaign_key TEXT CHECK (campaign_key IS NULL OR campaign_key ~ '^[A-Za-z0-9._-]{1,64}$'),
    flow TEXT NOT NULL CHECK (flow IN ('WEBMASTER_REGISTER','SITE_VERIFY','WIDGET_ENABLE','AGENCY_CREATE','ORG_CLAIM')),
    landing_path TEXT CHECK (landing_path IS NULL OR (length(landing_path) BETWEEN 1 AND 256 AND landing_path LIKE '/%')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    converted_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);
CREATE INDEX idx_growth_attribution_expiry ON growth_attribution_sessions(expires_at) WHERE converted_at IS NULL;
CREATE INDEX idx_growth_attribution_referral ON growth_attribution_sessions(referral_id,created_at);

CREATE TABLE growth_attribution_daily (
    daily_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_user_id BIGINT REFERENCES webmaster_users(user_id) ON DELETE CASCADE,
    referral_id BIGINT REFERENCES growth_referrals(referral_id) ON DELETE CASCADE,
    campaign_key TEXT NOT NULL DEFAULT '',
    flow TEXT NOT NULL,
    day DATE NOT NULL,
    starts BIGINT NOT NULL DEFAULT 0 CHECK (starts>=0),
    conversions BIGINT NOT NULL DEFAULT 0 CHECK (conversions>=0)
);
CREATE UNIQUE INDEX uq_growth_attribution_daily_dims ON growth_attribution_daily(owner_user_id,referral_id,campaign_key,flow,day) NULLS NOT DISTINCT;
