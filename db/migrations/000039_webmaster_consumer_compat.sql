CREATE OR REPLACE FUNCTION ensure_webmaster_consumer_identity()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    consumer_id BIGINT;
BEGIN
    IF NEW.consumer_user_id IS NOT NULL THEN
        RETURN NEW;
    END IF;

    SELECT user_id INTO consumer_id
    FROM consumer_users
    WHERE lower(email)=lower(NEW.email)
    FOR UPDATE;

    IF consumer_id IS NULL THEN
        INSERT INTO consumer_users(email,password_hash,status)
        VALUES(NEW.email,NEW.password_hash,CASE WHEN NEW.status='DISABLED' THEN 'DISABLED' ELSE 'ACTIVE' END)
        RETURNING user_id INTO consumer_id;
    END IF;

    NEW.consumer_user_id := consumer_id;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_webmaster_consumer_identity ON webmaster_users;
CREATE TRIGGER trg_webmaster_consumer_identity
BEFORE INSERT ON webmaster_users
FOR EACH ROW EXECUTE FUNCTION ensure_webmaster_consumer_identity();
