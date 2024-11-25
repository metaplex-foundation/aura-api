create table subscriptions
(
    sbs_id                 serial      not null
        constraint subscriptions_pk
            primary key,
    sbs_name                    varchar(32)                 not null,
    sbs_request_per_second      integer                     not null,
    sbs_minimum_balance         integer                     not null,
    sbs_tokens_limit            integer                     not null,
    sbs_created_at              timestamp default now()     not null
);
create unique index subscriptions_sbs_name_uindex
    on subscriptions (sbs_name);
INSERT INTO subscriptions (sbs_name, sbs_request_per_second, sbs_minimum_balance, sbs_tokens_limit)
VALUES ('free', 2, 0, 2);

create table users
(
    usr_id        bigserial
        constraint users_pk
            primary key,
    usr_dynamic_id      varchar(128) not null,
    usr_mplx_balance    bigint not null default 0,
    sbs_id bigint not null default 1
        references subscriptions
            on update cascade on delete restrict,
    usr_created_at      timestamp default now() not null
);
create unique index users_usr_dynamic_id_uindex
    on users (usr_dynamic_id);

create table networks
(
    ntw_id        bigserial
        constraint networks_pk
            primary key,
    ntw_name varchar(32) not null
);
create unique index networks_ntw_name_uindex
    on networks (ntw_name);

INSERT INTO networks (ntw_name)
VALUES ('solana'), ('aura');

create table user_api_keys
(
    uak_id        bigserial
        constraint user_api_keys_pk
            primary key,
    usr_id    bigint       not null
        constraint user_api_keys_users_usr_id_fk
            references users
            on update cascade on delete restrict,
    uak_name  varchar(128)          not null,
    uak_token uuid                  not null default gen_random_uuid(),
    uak_created_at timestamp default now()      not null,
    uak_deleted_at timestamp
);
CREATE INDEX user_api_usr_id_uak_deleted_at_index
    ON user_api_keys (usr_id, uak_deleted_at);
CREATE INDEX user_api_usr_uak_token_index
    ON user_api_keys (uak_token);
CREATE UNIQUE INDEX user_api_keys_usr_id_uak_name_unique_index
    ON user_api_keys (usr_id, uak_name)
    WHERE uak_deleted_at IS NULL;

create table user_api_keys_networks
(
    uak_id  bigint       not null
            references user_api_keys
            on update cascade on delete restrict,
    ntw_id  bigint       not null
        references networks
            on update cascade on delete restrict,
    primary key (uak_id, ntw_id)
);

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
        WHERE usr_id = NEW.usr_id AND uak_deleted_at IS NULL;

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
