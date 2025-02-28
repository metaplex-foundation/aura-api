-- Create a temporary table without the request_type column (to match original structure)
CREATE TABLE aura.user_subscription_usage_temp
(
    `time` DateTime,
    `chain` String,
    `user_uid` String,
    `tkn_uuid` UUID,
    `used_credits` Int64,
    `is_mainnet` Nullable(Bool)
)
ENGINE = ReplacingMergeTree
ORDER BY (time, user_uid, tkn_uuid, chain, is_mainnet)
SETTINGS allow_nullable_key = 1, index_granularity = 8192;

-- Migrate the data back with reverse transformations
-- Convert chain values back to their original values based on the request_type
INSERT INTO aura.user_subscription_usage_temp
SELECT
    time,
    -- Reverse the chain transformation based on request_type
    CASE
        WHEN chain = 'solana' AND request_type = 'RPC' THEN 'solana'
        WHEN chain = 'solana' AND request_type = 'DAS' THEN 'solana-das'
        WHEN chain = 'solana' AND request_type = 'GPA' THEN 'getProgramAccounts'
        WHEN chain = 'solana' AND request_type = 'SWQOS' THEN 'solana'
        WHEN chain = 'solana' AND request_type = 'Websocket' THEN 'solana'
        WHEN chain = 'eclipse' AND request_type = 'RPC' THEN 'eclipse'
        WHEN chain = 'eclipse' AND request_type = 'DAS' THEN 'eclipse-das'
        WHEN chain = 'eclipse' AND request_type = 'GPA' THEN 'eclipse'
        WHEN chain = 'eclipse' AND request_type = 'SWQOS' THEN 'eclipse'
        WHEN chain = 'eclipse' AND request_type = 'Websocket' THEN 'eclipse'
        ELSE chain
    END AS chain,
    user_uid,
    tkn_uuid,
    used_credits,
    is_mainnet
FROM aura.user_subscription_usage;

-- Drop the current table
DROP TABLE aura.user_subscription_usage;

-- Rename the temporary table to the original name
RENAME TABLE aura.user_subscription_usage_temp TO aura.user_subscription_usage;
