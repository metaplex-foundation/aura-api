ALTER TABLE user_api_keys ADD COLUMN deprecated boolean default false not null;

-- change function to respect deprecated flag
CREATE OR REPLACE FUNCTION check_api_key_limit()
RETURNS trigger AS $$
DECLARE
    token_limit INTEGER;
    active_key_count INTEGER;
BEGIN
    IF (TG_OP = 'INSERT') THEN
        SELECT s.sbs_tokens_limit INTO token_limit
        FROM subscriptions s
        JOIN users u ON u.sbs_id = s.sbs_id
        WHERE u.usr_id = NEW.usr_id;

        SELECT COUNT(*) INTO active_key_count
        FROM user_api_keys
        WHERE usr_id = NEW.usr_id AND uak_deleted_at IS NULL AND deprecated = false;

        IF active_key_count >= token_limit THEN
            RAISE EXCEPTION 'API keys limit reached';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER check_api_key_limit_trigger
    BEFORE INSERT ON user_api_keys
    FOR EACH ROW
    EXECUTE FUNCTION check_api_key_limit();
