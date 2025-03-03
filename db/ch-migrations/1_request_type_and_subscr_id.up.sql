-- Create the new table with identical structure as the original table
CREATE TABLE aura.stats_new
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
    `is_mainnet` UInt8,
    `request_type` String, -- Add the new request_type column
    `subscription_id` Int64 -- Add the new subscription_id column
)
ENGINE = ReplacingMergeTree
ORDER BY (user_uid, tkn_uuid, request_uuid)
SETTINGS index_granularity = 8192;

-- Migrate the data with transformations
-- First determine the request_type based on original chain values
-- Then set the chain values according to the requirements
INSERT INTO aura.stats_new
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
    -- Transform the chain value
    CASE
        WHEN original_chain = 'solana-das' THEN 'solana'
        WHEN original_chain = 'solana' THEN 'solana'
        WHEN original_chain = 'getProgramAccounts' THEN 'solana'
        WHEN original_chain = 'eclipse-das' THEN 'eclipse'
        WHEN original_chain = 'eclipse' THEN 'eclipse'
        ELSE original_chain
    END AS chain,
    response_size_bytes,
    target_type,
    is_mainnet,
    -- Set the request_type based on the ORIGINAL chain value
    CASE
        WHEN original_chain = 'solana' THEN 'RPC'
        WHEN original_chain = 'solana-das' THEN 'DAS'
        WHEN original_chain = 'getProgramAccounts' THEN 'GPA'
        WHEN original_chain = 'eclipse' THEN 'RPC'
        WHEN original_chain = 'eclipse-das' THEN 'DAS'
        ELSE 'RPC' -- Default value for any unexpected cases
    END AS request_type,
    CASE 
        WHEN method_cost > 0 THEN 2
        ELSE 1
    END AS subscription_id
FROM (
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
        chain AS original_chain,
        response_size_bytes,
        target_type,
        is_mainnet
    FROM aura.stats
) as original_data;

-- Drop the old table
DROP TABLE aura.stats;

-- Rename the new table to the original name
RENAME TABLE aura.stats_new TO aura.stats;
