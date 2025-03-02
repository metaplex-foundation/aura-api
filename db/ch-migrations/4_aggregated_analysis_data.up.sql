-- Create the new table with identical structure as the original table
CREATE TABLE aura.aggregated_analysis_data_new
(
    `chain` String,
    `request_type` String, -- Add the new request_type column
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
    request_type,
    is_mainnet,
    provider,
    rpc_method,
    rpc_request_data,
    day)
SETTINGS allow_nullable_key = 1, index_granularity = 8192;

-- Migrate the data with transformations
-- First determine the request_type based on original chain values
-- Then set the chain values according to the requirements
INSERT INTO aura.aggregated_analysis_data_new
SELECT
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
        ELSE 'RPC' -- Default value for any unexpected cases
    END AS request_type,
    rpc_method,
    rpc_request_data,
    provider,
    day,
    execution_time_ms,
    response_time_ms,
    p95_response_time_ms,
    total_req,
    is_mainnet
FROM (
    SELECT
        chain AS original_chain,
        rpc_method,
        rpc_request_data,
        provider,
        day,
        execution_time_ms,
        response_time_ms,
        p95_response_time_ms,
        total_req,
        is_mainnet
    FROM aura.aggregated_analysis_data
) as original_data;

-- Drop the old table
DROP TABLE aura.aggregated_analysis_data;

-- Rename the new table to the original name
RENAME TABLE aura.aggregated_analysis_data_new TO aura.aggregated_analysis_data;
