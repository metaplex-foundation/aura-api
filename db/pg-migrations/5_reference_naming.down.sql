ALTER TABLE crypto_payments RENAME COLUMN crp_memo TO crp_reference;

ALTER INDEX crypto_payments_crp_memo_idx RENAME TO crypto_payments_crp_reference_idx;
