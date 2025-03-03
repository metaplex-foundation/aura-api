-- Create a temporary table without the request_type column (to match original structure)
CREATE TABLE aura.aggregated_user_hourly_data_temp
(
    `user_uid` String,
    `tkn_uuid` UUID,
    `timestamp` DateTime,
    `rpc_method` String,
    `chain` String,
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
ORDER BY (user_uid, tkn_uuid, timestamp, rpc_method, chain, is_mainnet)
SETTINGS allow_nullable_key = 1, index_granularity = 8192;

-- Migrate the data back with reverse transformations
-- Convert chain values back to their original values based on the request_type
INSERT INTO aura.aggregated_user_hourly_data_temp
SELECT
    user_uid,
    tkn_uuid,
    timestamp,
    rpc_method,
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
    total_req,
    success_req,
    http_err,
    rpc_err,
    response_size_bytes,
    avg_response_time_ms,
    p95_response_time_ms,
    is_mainnet
FROM aura.aggregated_user_hourly_data;

-- Drop the current table
DROP TABLE aura.aggregated_user_hourly_data;

-- Rename the temporary table to the original name
RENAME TABLE aura.aggregated_user_hourly_data_temp TO aura.aggregated_user_hourly_data;
