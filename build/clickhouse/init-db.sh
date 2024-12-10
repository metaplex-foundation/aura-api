#!/bin/bash
set -e

clickhouse client -n <<-EOSQL
    CREATE DATABASE IF NOT EXISTS aura;

    CREATE TABLE IF NOT EXISTS aura.stats(
        user_uid String,
        tkn_uuid UUID,
        request_uuid UUID,
        status UInt16,
        execution_time_ms Int64,
        endpoint String,
        attempts UInt8,
        response_time_ms Int64,
        rpc_error_code String,
        user_agent String,
        rpc_method String,
        rpc_request_data String,
        timestamp DateTime,
        server_id String,
        provider String,
        method_cost String,
        chain String,
        response_size_bytes Int64,
        target_type String
    ) ENGINE = ReplacingMergeTree()
          ORDER BY (user_uid, tkn_uuid, request_uuid);

    create table if not exists aura.aggregated_analysis_data (
        chain String,
        rpc_method String,
        rpc_request_data String,
        day Date,
        execution_time_ms Int64,
        response_time_ms Int64,
        p95_response_time_ms Int64,
        total_req UInt64
    ) ENGINE = ReplacingMergeTree()
          ORDER BY (chain, rpc_method, rpc_request_data, day);

    create table if not exists aura.aggregated_user_daily_data (
            user_uid String,
            tkn_uuid UUID,
            day Date,
            rpc_method String,
            chain String,
            total_req UInt64,
            success_req UInt64,
            http_err UInt64,
            rpc_err UInt64,
            response_size_bytes Int64,
            avg_response_time_ms Int64,
            p95_response_time_ms Int64
    ) ENGINE = ReplacingMergeTree()
          ORDER BY (user_uid, tkn_uuid, day, rpc_method, chain);

    create table if not exists aura.aggregated_user_hourly_data (
        user_uid String,
        tkn_uuid UUID,
        timestamp DATETIME,
        rpc_method String,
        chain String,
        total_req UInt64,
        success_req UInt64,
        http_err UInt64,
        rpc_err UInt64,
        response_size_bytes Int64,
        avg_response_time_ms Int64,
        p95_response_time_ms Int64
    ) ENGINE = ReplacingMergeTree()
          ORDER BY (user_uid, tkn_uuid, timestamp, rpc_method, chain);

    create table if not exists aura.user_subscription_usage
    (
        time         DateTime,
        chain        String,
        user_uid     String,
        tkn_uuid     UUID,
        used_credits Int64
    ) ENGINE = ReplacingMergeTree()
          ORDER BY (time, user_uid, tkn_uuid, chain);
EOSQL