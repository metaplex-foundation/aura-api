ALTER TABLE users 
    ADD COLUMN IF NOT EXISTS usr_next_sbs_id bigint default 0
        references subscriptions
            on update cascade on delete restrict;

DROP TRIGGER IF EXISTS trg_check_and_update_subscription ON users;
