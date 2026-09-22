CREATE OR REPLACE FUNCTION ensure_webmaster_consumer_identity()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    linked_id BIGINT;
BEGIN
    IF NEW.consumer_user_id IS NOT NULL THEN
        RETURN NEW;
    END IF;

    SELECT user_id INTO linked_id
    FROM consumer_users
    WHERE lower(email)=lower(NEW.email)
    FOR UPDATE;

    IF linked_id IS NULL THEN
        INSERT INTO consumer_users(email,password_hash,status,created_at,updated_at)
        VALUES(
            NEW.email,
            NEW.password_hash,
            CASE WHEN NEW.status='DISABLED' THEN 'DISABLED' ELSE 'ACTIVE' END,
            COALESCE(NEW.created_at,now()),
            COALESCE(NEW.updated_at,now())
        )
        RETURNING user_id INTO linked_id;
    END IF;

    NEW.consumer_user_id := linked_id;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_webmaster_consumer_identity ON webmaster_users;
CREATE TRIGGER trg_webmaster_consumer_identity
BEFORE INSERT ON webmaster_users
FOR EACH ROW
WHEN (NEW.consumer_user_id IS NULL)
EXECUTE FUNCTION ensure_webmaster_consumer_identity();
