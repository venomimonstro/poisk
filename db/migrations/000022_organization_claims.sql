CREATE TABLE organization_claims (
    claim_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id BIGINT NOT NULL REFERENCES organizations(place_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE CASCADE,
    site_id BIGINT NOT NULL REFERENCES webmaster_sites(site_id) ON DELETE CASCADE,
    proof_type TEXT NOT NULL CHECK (proof_type IN ('VERIFIED_WEBSITE_HOST')),
    proof_host TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','REVOKED')),
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(place_id,user_id,site_id)
);
CREATE UNIQUE INDEX idx_organization_claims_active_place ON organization_claims(place_id) WHERE status='ACTIVE';
CREATE INDEX idx_organization_claims_user_active ON organization_claims(user_id,place_id) WHERE status='ACTIVE';

CREATE TABLE organization_claim_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    claim_id BIGINT REFERENCES organization_claims(claim_id) ON DELETE SET NULL,
    place_id BIGINT NOT NULL REFERENCES organizations(place_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES webmaster_users(user_id) ON DELETE RESTRICT,
    site_id BIGINT REFERENCES webmaster_sites(site_id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('CLAIM','REVOKE')),
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_organization_claim_events_place ON organization_claim_events(place_id,created_at DESC);
