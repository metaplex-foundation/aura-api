CREATE TABLE providers_rewards
(
    prw_provider VARCHAR(100) NOT NULL,
    prw_rewards BIGINT NOT NULL,
    prw_day DATE NOT NULL,
    PRIMARY KEY(prw_provider, prw_day)
);
