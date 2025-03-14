DROP INDEX IF EXISTS user_api_usr_id_uak_deleted_at_index;

CREATE INDEX user_api_usr_id_uak_deleted_at_index
    ON user_api_keys (usr_id, uak_deleted_at, deprecated);
