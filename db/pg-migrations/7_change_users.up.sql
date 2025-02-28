ALTER TABLE users 
    ADD COLUMN IF NOT EXISTS usr_next_sbs_id bigint default 0
        references subscriptions
            on update cascade on delete restrict;