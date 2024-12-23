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
CREATE UNIQUE INDEX crypto_payments_crp_reference_idx ON crypto_payments (crp_reference);
CREATE UNIQUE INDEX crypto_payments_crp_signature_idx ON crypto_payments (crp_signature);
CREATE INDEX idx_crypto_payments_usr_id ON crypto_payments (usr_id);
CREATE INDEX idx_crypto_payments_created_at ON crypto_payments (crp_created_at DESC);
CREATE INDEX idx_crypto_payments_paid_at ON crypto_payments (crp_paid_at);

create table failed_processing_transaction
(
    fpt_id        bigserial
        constraint failed_processing_transaction_pk
            primary key,
    fpt_signature       varchar(100),
    fpt_fail_reason     text,
    fpt_created_at      timestamp default now() not null
);
CREATE UNIQUE INDEX idx_failed_processing_transaction_signature ON failed_processing_transaction (fpt_signature);
CREATE INDEX idx_failed_processing_transaction_created_at ON failed_processing_transaction (fpt_created_at);