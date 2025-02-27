-- Create the new table with identical structure as the original table
CREATE TABLE aura.user_subscription_usage_new
(
    `time` DateTime,
    `request_type` String, -- Add the new request_type column
    `chain` String,
    `user_uid` String,
    `tkn_uuid` UUID,
    `used_credits` Int64,
    `is_mainnet` Nullable(Bool)
)
ENGINE = ReplacingMergeTree
ORDER BY (time, user_uid, tkn_uuid, chain, is_mainnet)
SETTINGS allow_nullable_key = 1, index_granularity = 8192;

-- Migrate the data with transformations
-- First determine the request_type based on original chain values
-- Then set the chain values according to the requirements
INSERT INTO aura.user_subscription_usage_new
SELECT
    time,
    -- Set the request_type based on the ORIGINAL chain value
    CASE
        WHEN original_chain = 'solana' THEN 'RPC'
        WHEN original_chain = 'solana-das' THEN 'DAS'
        WHEN original_chain = 'getProgramAccounts' THEN 'GPA'
        WHEN original_chain = 'eclipse' THEN 'RPC'
        WHEN original_chain = 'eclipse-das' THEN 'DAS'
        ELSE 'RPC' -- Default value for any unexpected cases
    END AS request_type,
    -- Transform the chain value
    CASE
        WHEN original_chain = 'solana-das' THEN 'solana'
        WHEN original_chain = 'solana' THEN 'solana'
        WHEN original_chain = 'getProgramAccounts' THEN 'solana'
        WHEN original_chain = 'eclipse-das' THEN 'eclipse'
        WHEN original_chain = 'eclipse' THEN 'eclipse'
        ELSE original_chain
    END AS chain,
    user_uid,
    tkn_uuid,
    used_credits,
    is_mainnet
FROM (
    SELECT
        time,
        chain AS original_chain,
        user_uid,
        tkn_uuid,
        used_credits,
        is_mainnet
    FROM aura.user_subscription_usage
) as original_data;

-- Drop the old table
DROP TABLE aura.user_subscription_usage;

-- Rename the new table to the original name
RENAME TABLE aura.user_subscription_usage_new TO aura.user_subscription_usage;
