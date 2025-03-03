-- Create a temporary table without the request_type column (to match original structure)
CREATE TABLE aura.stats_temp
(
    `user_uid` String,
    `tkn_uuid` UUID,
    `request_uuid` UUID,
    `status` UInt16,
    `execution_time_ms` Int64,
    `endpoint` String,
    `attempts` UInt8,
    `response_time_ms` Int64,
    `rpc_error_code` String,
    `user_agent` String,
    `rpc_method` String,
    `rpc_request_data` String,
    `timestamp` DateTime,
    `server_id` String,
    `provider` String,
    `method_cost` Int64,
    `chain` String,
    `response_size_bytes` Int64,
    `target_type` String,
    `is_mainnet` UInt8
)
ENGINE = ReplacingMergeTree
ORDER BY (user_uid, tkn_uuid, request_uuid)
SETTINGS index_granularity = 8192;

-- Migrate the data back with reverse transformations
-- Convert chain values back to their original values based on the request_type
INSERT INTO aura.stats_temp
SELECT
    user_uid,
    tkn_uuid,
    request_uuid,
    status,
    execution_time_ms,
    endpoint,
    attempts,
    response_time_ms,
    rpc_error_code,
    user_agent,
    rpc_method,
    rpc_request_data,
    timestamp,
    server_id,
    provider,
    method_cost,
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
    response_size_bytes,
    target_type,
    is_mainnet
FROM aura.stats;

-- Drop the current table
DROP TABLE aura.stats;

-- Rename the temporary table to the original name
RENAME TABLE aura.stats_temp TO aura.stats;
