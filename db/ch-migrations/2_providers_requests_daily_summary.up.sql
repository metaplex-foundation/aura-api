create table if not exists aura.providers_requests_daily_summary
(
    provider String,
    request_type String,
    chain String,
    payment_plan Enum('pay-as-you-go' = 1, 'subscription' = 2),
    price_per_request Int64,
    num_of_requests Int64,
    day Date
) ENGINE = ReplacingMergeTree()
    ORDER BY (payment_plan, provider, request_type, chain, day)
    SETTINGS allow_nullable_key = 1;
