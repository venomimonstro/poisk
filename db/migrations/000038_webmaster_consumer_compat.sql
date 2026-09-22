CREATE OR REPLACE FUNCTION ensure_webmaster_consumer_identity()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    linked_id BIGINT;
BEGIN
    IF NEW.consumer_user_id IS NOT NULL THEN
        RETURN NEW;
    END IF;

    INSERT INTO consumer_users(email,password_hash,status,created_at,updated_at)
    VALUES(NEW.email,NEW.password_hash,NEW.status,COALESCE(NEW.created_at,now()),COALESCE(NEW.updated_at,now()))
    RETURNING user_id INTO linked_id;

    NEW.consumer_user_id := linked_id;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_webmaster_consumer_identity
BEFORE INSERT ON webmaster_users
FOR EACH ROW
WHEN (NEW.consumer_user_id IS NULL)
EXECUTE FUNCTION ensure_webmaster_consumer_identity();
