create table crypto_payments
(
    crp_id        bigserial
        constraint crypto_payments_pk
            primary key,
    crp_reference      varchar(50) not null,
    crp_signature      varchar(100),
    usr_id bigint not null default 1
        references users
            on update cascade on delete restrict,
    crp_mplx_amount     bigint,
    crp_created_at      timestamp default now() not null,
    crp_paid_at         timestamp
);

create table failed_processing_transaction
(
    fpt_id        bigserial
        constraint failed_processing_transaction_pk
            primary key,
    fpt_signature       varchar(100),
    fpt_fail_reason     text,
    fpt_created_at      timestamp default now() not null
);