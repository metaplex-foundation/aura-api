-- Create the new table with identical structure as the original table

CREATE TABLE aura.aggregated_user_daily_data_new
(
    `user_uid` String,
    `tkn_uuid` UUID,
    `day` Date,
    `rpc_method` String,
    `chain` String,
    `request_type` String, -- Add the new request_type column
    `total_req` UInt64,
    `success_req` UInt64,
    `http_err` UInt64,
    `rpc_err` UInt64,
    `response_size_bytes` Int64,
    `avg_response_time_ms` Int64,
    `p95_response_time_ms` Int64,
    `is_mainnet` Nullable(Bool)
)
ENGINE = ReplacingMergeTree
ORDER BY (user_uid, tkn_uuid, day, rpc_method, chain, request_type, is_mainnet)
SETTINGS allow_nullable_key = 1, index_granularity = 8192;

-- Migrate the data with transformations
-- First determine the request_type based on original chain values
-- Then set the chain values according to the requirements
INSERT INTO aura.aggregated_user_daily_data_new
SELECT
    user_uid,
    tkn_uuid,
    day,
    rpc_method,
    -- Transform the chain value
    CASE
        WHEN original_chain = 'solana-das' THEN 'solana'
        WHEN original_chain = 'solana' THEN 'solana'
        WHEN original_chain = 'getProgramAccounts' THEN 'solana'
        WHEN original_chain = 'eclipse-das' THEN 'eclipse'
        WHEN original_chain = 'eclipse' THEN 'eclipse'
        ELSE original_chain
    END AS chain,
    -- Set the request_type based on the ORIGINAL chain value
    CASE
        WHEN original_chain = 'solana' THEN 'RPC'
        WHEN original_chain = 'solana-das' THEN 'DAS'
        WHEN original_chain = 'getProgramAccounts' THEN 'GPA'
        WHEN original_chain = 'eclipse' THEN 'RPC'
        WHEN original_chain = 'eclipse-das' THEN 'DAS'
        ELSE original_chain
    END AS request_type,
    total_req,
    success_req,
    http_err,
    rpc_err,
    response_size_bytes,
    avg_response_time_ms,
    p95_response_time_ms,
    is_mainnet
FROM (
    SELECT
        user_uid,
        tkn_uuid,
        day,
        rpc_method,
        chain AS original_chain,
        total_req,
        success_req,
        http_err,
        rpc_err,
        response_size_bytes,
        avg_response_time_ms,
        p95_response_time_ms,
        is_mainnet
    FROM aura.aggregated_user_daily_data
) as original_data;

-- Drop the old table
DROP TABLE aura.aggregated_user_daily_data;

-- Rename the new table to the original name
RENAME TABLE aura.aggregated_user_daily_data_new TO aura.aggregated_user_daily_data;
