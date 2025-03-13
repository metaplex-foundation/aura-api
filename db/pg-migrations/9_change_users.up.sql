ALTER TABLE users
    ADD COLUMN IF NOT EXISTS usr_next_sbs_id bigint default 1
        references subscriptions(sbs_id)
            on update cascade on delete restrict;

DROP TRIGGER IF EXISTS trg_check_and_update_subscription ON users;
