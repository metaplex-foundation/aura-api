ALTER TABLE crypto_payments RENAME COLUMN crp_reference TO crp_memo;

ALTER INDEX crypto_payments_crp_reference_idx RENAME TO crypto_payments_crp_memo_idx;
