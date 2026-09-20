CREATE OR REPLACE FUNCTION webmaster_bound_sessions() RETURNS trigger AS $$
BEGIN
    DELETE FROM webmaster_sessions
    WHERE user_id = NEW.user_id AND expires_at <= now();

    WHILE (SELECT count(*) FROM webmaster_sessions WHERE user_id = NEW.user_id) >= 10 LOOP
        DELETE FROM webmaster_sessions
        WHERE session_id = (
            SELECT session_id
            FROM webmaster_sessions
            WHERE user_id = NEW.user_id
            ORDER BY last_seen_at ASC, session_id ASC
            LIMIT 1
        );
    END LOOP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_webmaster_bound_sessions
BEFORE INSERT ON webmaster_sessions
FOR EACH ROW EXECUTE FUNCTION webmaster_bound_sessions();
