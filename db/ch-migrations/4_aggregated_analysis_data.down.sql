-- Create a temporary table without the request_type column (to match original structure)
CREATE TABLE aura.aggregated_analysis_data_temp
(
    `chain` String,
    `rpc_method` String,
    `rpc_request_data` String,
    `provider` String,
    `day` Date,
    `execution_time_ms` Int64,
    `response_time_ms` Int64,
    `p95_response_time_ms` Int64,
    `total_req` UInt64,
    `is_mainnet` Nullable(Bool)
)
ENGINE = ReplacingMergeTree
ORDER BY (chain,
    is_mainnet,
    provider,
    rpc_method,
    rpc_request_data,
    day)
SETTINGS allow_nullable_key = 1, index_granularity = 8192;

-- Migrate the data back with reverse transformations
-- Convert chain values back to their original values based on the request_type
INSERT INTO aura.aggregated_analysis_data_temp
SELECT
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
    rpc_method,
    rpc_request_data,
    provider,
    day,
    execution_time_ms,
    response_time_ms,
    p95_response_time_ms,
    total_req,
    is_mainnet
FROM aura.aggregated_analysis_data;

-- Drop the current table
DROP TABLE aura.aggregated_analysis_data;

-- Rename the temporary table to the original name
RENAME TABLE aura.aggregated_analysis_data_temp TO aura.aggregated_analysis_data;
