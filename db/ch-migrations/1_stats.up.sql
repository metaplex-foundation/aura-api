create table if not exists aura.aggregated_usage_data
(
    time DateTime,
    users_total Int64,
    total_free_subscriptions Int64,
    total_developer_subscriptions Int64,
    total_advanced_subscriptions Int64,
    total_pro_subscriptions Int64,
    total_not_used_mplx Int64
) ENGINE = ReplacingMergeTree()
    ORDER BY (time)
    SETTINGS allow_nullable_key = 1;

create table if not exists aura.aggregated_providers_stats
(
    time DateTime,
    provider String,
    chain String,
    is_mainnet bool,
    total_free_requests Int64,
    total_paid_requests Int64
) ENGINE = ReplacingMergeTree()
    ORDER BY (time, provider, chain)
    SETTINGS allow_nullable_key = 1;

alter table aura.stats add column subscription_id Int64 after is_mainnet;