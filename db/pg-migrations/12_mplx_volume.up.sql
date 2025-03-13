CREATE INDEX idx_crypto_payments_crp_status ON public.crypto_payments USING btree (crp_status);

CREATE TABLE rewards_transactions
(
    rwd_id BIGSERIAL PRIMARY KEY,
    rwd_provider VARCHAR(100) NOT NULL,
    rwd_rewards_day DATE NOT NULL,
    rwd_paid_at TIMESTAMP DEFAULT now(),
    rwd_transaction_signature VARCHAR(100) NOT NULL,
    
    CONSTRAINT fk_providers_rewards
        FOREIGN KEY (rwd_provider, rwd_rewards_day)
        REFERENCES providers_rewards(prw_provider, prw_day)
        ON UPDATE CASCADE    -- Changes to provider name or date will cascade
        ON DELETE NO ACTION  -- If any of rows deleted another part of data should stay
);

CREATE UNIQUE INDEX idx_rewards_transactions_unique_provider_day
ON rewards_transactions(rwd_provider, rwd_rewards_day);
