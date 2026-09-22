CREATE TABLE organization_reviews (
    review_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id BIGINT NOT NULL REFERENCES organizations(place_id) ON DELETE CASCADE,
    consumer_user_id BIGINT NOT NULL REFERENCES consumer_users(user_id) ON DELETE RESTRICT,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body TEXT NOT NULL CHECK (length(body) BETWEEN 10 AND 4000),
    status TEXT NOT NULL DEFAULT 'VISIBLE' CHECK (status IN ('VISIBLE','PENDING','HIDDEN','REJECTED','DELETED')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    change_actor_type TEXT NOT NULL DEFAULT 'USER' CHECK (change_actor_type IN ('USER','ADMIN','SYSTEM')),
    change_actor_id BIGINT,
    change_reason TEXT NOT NULL DEFAULT 'USER_CREATE' CHECK (length(change_reason) BETWEEN 2 AND 160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(place_id,consumer_user_id)
);
CREATE INDEX idx_org_reviews_place_visible ON organization_reviews(place_id,created_at DESC,review_id DESC) WHERE status='VISIBLE';
CREATE INDEX idx_org_reviews_status ON organization_reviews(status,updated_at DESC,review_id DESC);

CREATE TABLE organization_review_revisions (
    revision_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    review_id BIGINT NOT NULL REFERENCES organization_reviews(review_id) ON DELETE CASCADE,
    version BIGINT NOT NULL CHECK (version > 0),
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body TEXT NOT NULL CHECK (length(body) BETWEEN 10 AND 4000),
    status TEXT NOT NULL CHECK (status IN ('VISIBLE','PENDING','HIDDEN','REJECTED','DELETED')),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('USER','ADMIN','SYSTEM')),
    actor_id BIGINT,
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 2 AND 160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(review_id,version)
);
CREATE INDEX idx_org_review_revisions_review ON organization_review_revisions(review_id,version DESC);

CREATE OR REPLACE FUNCTION organization_review_version_and_snapshot() RETURNS trigger AS $$
BEGIN
    IF TG_OP='UPDATE' THEN
        NEW.version := OLD.version + 1;
        NEW.updated_at := now();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_version BEFORE UPDATE ON organization_reviews FOR EACH ROW EXECUTE FUNCTION organization_review_version_and_snapshot();

CREATE OR REPLACE FUNCTION organization_review_snapshot_after() RETURNS trigger AS $$
BEGIN
    INSERT INTO organization_review_revisions(review_id,version,rating,body,status,actor_type,actor_id,reason)
    VALUES(NEW.review_id,NEW.version,NEW.rating,NEW.body,NEW.status,NEW.change_actor_type,COALESCE(NEW.change_actor_id,CASE WHEN NEW.change_actor_type='USER' THEN NEW.consumer_user_id ELSE NULL END),NEW.change_reason);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_snapshot AFTER INSERT OR UPDATE ON organization_reviews FOR EACH ROW EXECUTE FUNCTION organization_review_snapshot_after();

CREATE OR REPLACE FUNCTION organization_review_revisions_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'organization_review_revisions is immutable';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_revisions_no_update BEFORE UPDATE OR DELETE ON organization_review_revisions FOR EACH ROW EXECUTE FUNCTION organization_review_revisions_immutable();

CREATE TABLE organization_review_stats (
    place_id BIGINT PRIMARY KEY REFERENCES organizations(place_id) ON DELETE CASCADE,
    review_count BIGINT NOT NULL DEFAULT 0 CHECK (review_count >= 0),
    rating_sum BIGINT NOT NULL DEFAULT 0 CHECK (rating_sum >= 0),
    average_rating NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (average_rating BETWEEN 0 AND 5),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION refresh_organization_review_stats(p_place_id BIGINT) RETURNS void AS $$
BEGIN
    INSERT INTO organization_review_stats(place_id,review_count,rating_sum,average_rating,updated_at)
    SELECT p_place_id,count(*),COALESCE(sum(rating),0),COALESCE(round(avg(rating)::numeric,2),0),now()
    FROM organization_reviews WHERE place_id=p_place_id AND status='VISIBLE'
    ON CONFLICT(place_id) DO UPDATE SET review_count=EXCLUDED.review_count,rating_sum=EXCLUDED.rating_sum,average_rating=EXCLUDED.average_rating,updated_at=now();
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION organization_review_stats_trigger() RETURNS trigger AS $$
BEGIN
    PERFORM refresh_organization_review_stats(COALESCE(NEW.place_id,OLD.place_id));
    IF TG_OP='UPDATE' AND NEW.place_id<>OLD.place_id THEN PERFORM refresh_organization_review_stats(OLD.place_id); END IF;
    RETURN COALESCE(NEW,OLD);
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_org_review_stats AFTER INSERT OR UPDATE OR DELETE ON organization_reviews FOR EACH ROW EXECUTE FUNCTION organization_review_stats_trigger();

CREATE TABLE organization_review_reports (
    report_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    review_id BIGINT NOT NULL REFERENCES organization_reviews(review_id) ON DELETE CASCADE,
    reporter_user_id BIGINT NOT NULL REFERENCES consumer_users(user_id) ON DELETE RESTRICT,
    reason TEXT NOT NULL CHECK (reason IN ('SPAM','ABUSE','FAKE','CONFLICT','OTHER')),
    details TEXT CHECK (details IS NULL OR length(details) <= 1000),
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','RESOLVED','DISMISSED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    resolved_by_admin_id BIGINT REFERENCES admin_users(admin_id) ON DELETE SET NULL,
    CHECK ((status='OPEN' AND resolved_at IS NULL) OR status<>'OPEN')
);
CREATE UNIQUE INDEX uq_org_review_report_open ON organization_review_reports(review_id,reporter_user_id) WHERE status='OPEN';
CREATE INDEX idx_org_review_reports_queue ON organization_review_reports(status,created_at,report_id);

CREATE TABLE organization_review_replies (
    reply_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    review_id BIGINT NOT NULL UNIQUE REFERENCES organization_reviews(review_id) ON DELETE CASCADE,
    claim_id BIGINT NOT NULL REFERENCES organization_claims(claim_id) ON DELETE RESTRICT,
    owner_consumer_user_id BIGINT NOT NULL REFERENCES consumer_users(user_id) ON DELETE RESTRICT,
    body TEXT NOT NULL CHECK (length(body) BETWEEN 2 AND 3000),
    status TEXT NOT NULL DEFAULT 'VISIBLE' CHECK (status IN ('VISIBLE','DELETED')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_org_review_replies_visible ON organization_review_replies(review_id) WHERE status='VISIBLE';

CREATE TABLE organization_review_moderation_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    review_id BIGINT NOT NULL REFERENCES organization_reviews(review_id) ON DELETE CASCADE,
    admin_id BIGINT REFERENCES admin_users(admin_id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('HIDE','SHOW','REJECT','REPORT_RESOLVE','REPORT_DISMISS')),
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 2 AND 240),
    details JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_org_review_moderation_review ON organization_review_moderation_events(review_id,event_id DESC);
