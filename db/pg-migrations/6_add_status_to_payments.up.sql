CREATE TYPE payment_status AS ENUM (
    'paid',
    'unpaid',
    'cancelled'
);

alter table crypto_payments 
    add column crp_status payment_status not null default 'unpaid';

update crypto_payments
    set crp_status = 'paid'
    where crp_signature is not null AND crp_paid_at is not null;

update crypto_payments
set crp_status = 'cancelled'
where crp_status = 'unpaid';
