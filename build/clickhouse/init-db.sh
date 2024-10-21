#!/bin/bash
set -e

clickhouse client -n <<-EOSQL
    CREATE DATABASE IF NOT EXISTS aura;

    CREATE TABLE IF NOT EXISTS aura.stats(
        user_uid String,
        prj_uuid UUID,
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
        chain String,
        response_size_bytes Int64,
        target_type String
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (user_uid, prj_uuid, request_uuid);

    CREATE TABLE IF NOT EXISTS aura.scanner_methods(
        server_id String,
        time DateTime,
        peer String,
        method String,
        time_connect_ms Int64,
        time_response_ms Int64,
        response_code UInt16,
        response_valid Bool
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (server_id, time, peer, method);

    CREATE TABLE IF NOT EXISTS aura.scanner_peers(
        server_id String,
        time DateTime,
        peer String,
        time_connect_ms Int64,
        is_alive Bool,
        is_full_history Bool,
        slot_amount UInt64
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (server_id, time, peer);

    create table if not exists aura.aggregated_analysis_data (
        chain String,
        rpc_method String,
        rpc_request_data String,
        day Date,
        execution_time_ms Int64,
        response_time_ms Int64,
        total_req UInt64
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (chain, rpc_method, rpc_request_data, day);

    create table if not exists aura.aggregated_user_data (
            user_uid String,
            prj_uuid UUID,
            day Date,
            rpc_method String,
            chain String,
            total_req UInt64,
            success_req UInt64,
            http_err UInt64,
            rpc_err UInt64,
            response_size_bytes Int64
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (user_uid, prj_uuid, day, rpc_method, chain);

    create table if not exists aura.user_subscription_usage
    (
        time         DateTime,
        user_uid     String,
        used_credits Int64
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (time, user_uid);

    CREATE TABLE IF NOT EXISTS aura.detailed_requests(
        user_uid String,
        request_uuid UUID,
        status UInt16,
        execution_time_ms Int64,
        endpoint String,
        attempts UInt8,
        rpc_error_code String,
        user_agent String,
        rpc_method String,
        rpc_request_body String,
        timestamp DateTime,
        server_id String,
        chain String,
        response_size_bytes Int64
    ) ENGINE = ReplacingMergeTree()
      ORDER BY (user_uid, request_uuid);
EOSQL