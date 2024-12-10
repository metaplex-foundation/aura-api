package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/proto"
	"github.com/adm-metaex/aura-api/pkg/util"
)

const (
	allData         = "All"
	hourlyThreshold = 1 * time.Hour
	dailyThreshold  = 24 * time.Hour
)

const (
	HourlyGranularity = "1h"
	DailyGranularity  = "1d"
)

type (
	ResponseTimeHistory struct {
		Timestamp         time.Time  `json:"timestamp"`
		AvgResponseTimeMs int64      `json:"avg_response_time_ms"`
		P95ResponseTimeMs int64      `json:"p95_response_time_ms"`
		RpcMethod         string     `json:"rpc_method"`
		Network           string     `json:"network"`
		Token             *uuid.UUID `json:"token,omitempty"`
	}
	RequestsVolumeHistory struct {
		Timestamp time.Time  `json:"timestamp"`
		Volume    int64      `json:"volume"`
		RpcMethod string     `json:"rpc_method"`
		Network   string     `json:"network"`
		Token     *uuid.UUID `json:"token,omitempty"`
	}
)

func (s *Storage) BatchInsertStats(stats []*proto.Stat) error {
	if len(stats) == 0 {
		return nil
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("tx begin error: %s", err)
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone { //nolint:errorlint
			log.Logger.General.Errorf("tx rollback error: %s", err)
		}
	}()

	stmt, err := tx.Prepare(`INSERT INTO stats (
		user_uid,
		prj_uuid,
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
        chain,
        response_size_bytes,
        target_type
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	for _, stat := range stats {
		_, err = stmt.Exec(
			stat.GetUserUid(),
			util.ParseUUIDOrDefault(stat.GetProjectUuid()),
			stat.GetRequestUuid(),
			uint16(stat.GetStatus()),
			stat.GetExecutionTimeMs(),
			stat.GetEndpoint(),
			uint8(stat.GetAttempts()),
			stat.GetResponseTimeMs(),
			stat.GetRpcErrorCode(),
			stat.GetUserAgent(),
			stat.GetRpcMethod(),
			stat.GetRpcRequestData(),
			stat.GetTimestamp().AsTime(),
			s.serverID,
			stat.GetChain(),
			stat.GetResponseSizeBytes(),
			stat.GetTargetType(),
		)
		if err != nil {
			return fmt.Errorf("exec statement error: %s", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("tx commit error: %s", err)
	}

	return nil
}

func (s *Storage) DeleteOutdatedStats(ctx context.Context) error {
	query := `ALTER TABLE aura.stats
    DELETE WHERE timestamp < now() - INTERVAL 7 DAY;`

	_, err := s.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	return nil
}

func (s *Storage) DeleteOutdatedHourlyData(ctx context.Context) error {
	query := `ALTER TABLE aura.aggregated_user_hourly_data
    DELETE WHERE timestamp < now() - INTERVAL 30 DAY;`

	_, err := s.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	return nil
}

func buildWhereCondition(builder sq.SelectBuilder, userUID string, tknUUID *uuid.UUID, chain *string, rpcMethod *string, isFromStats bool) sq.SelectBuilder {
	builder = builder.Where(sq.Eq{"user_uid": userUID})

	var tkn uuid.UUID
	if tknUUID != nil {
		tkn = *tknUUID
	}
	if !(isFromStats && tknUUID == nil) {
		builder = builder.Where(sq.Eq{"tkn_uuid": tkn})
	}

	ch := allData
	if chain != nil {
		ch = *chain
	}
	// TODO: refactor
	if !(isFromStats && ch == allData) && !(!isFromStats && rpcMethod != nil) {
		builder = builder.Where(sq.Eq{"chain": ch})
	}

	rm := allData
	if rpcMethod != nil {
		rm = *rpcMethod
	}
	if !(isFromStats && rm == allData) {
		builder = builder.Where(sq.Eq{"rpc_method": rm})
	}

	return builder
}

func (s *Storage) GetRequestsVolumeHistory(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
) (result []RequestsVolumeHistory, err error) {
	if userUID == "" {
		return nil, ErrEmptyUserUUID
	}
	diff := time.Since(startTime)
	isAggregated := (diff > dailyThreshold && granularity == DailyGranularity) || (diff > hourlyThreshold && granularity == HourlyGranularity)

	var sqlQuery string
	var args []interface{}
	if isAggregated {
		oldSQL, oldArgs, err := s.buildAggregatedQuery(userUID, tknUUID, chain, rpcMethod, granularity, startTime)
		if err != nil {
			return nil, fmt.Errorf("buildAggregatedQuery: %s", err)
		}

		newSQL, newArgs, err := s.buildStatsQuery(userUID, tknUUID, chain, rpcMethod, granularity)
		if err != nil {
			return nil, fmt.Errorf("buildStatsQuery 1: %s", err)
		}

		sqlQuery = fmt.Sprintf("SELECT * FROM (%s UNION ALL %s) AS combined ORDER BY ts", oldSQL, newSQL)
		args = append(args, oldArgs...)
		args = append(args, newArgs...)
	} else {
		sqlQuery, args, err = s.buildStatsQuery(userUID, tknUUID, chain, rpcMethod, granularity)
		if err != nil {
			return nil, fmt.Errorf("buildStatsQuery 2: %s", err)
		}
	}

	rows, err := s.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry RequestsVolumeHistory
		if err = rows.Scan(&entry.RpcMethod, &entry.Network, &entry.Token, &entry.Volume, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		var defaultUUID uuid.UUID
		if entry.Token != nil && *entry.Token == defaultUUID {
			entry.Token = nil
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Storage) GetResponseTimeHistory(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
) (result []ResponseTimeHistory, err error) {
	if userUID == "" {
		return nil, ErrEmptyUserUUID
	}
	diff := time.Since(startTime)
	isAggregated := (diff > dailyThreshold && granularity == DailyGranularity) || (diff > hourlyThreshold && granularity == HourlyGranularity)

	var sqlQuery string
	var args []interface{}
	if isAggregated {
		oldSQL, oldArgs, err := s.buildAggregatedQuery(userUID, tknUUID, chain, rpcMethod, granularity, startTime)
		if err != nil {
			return nil, fmt.Errorf("buildAggregatedQuery: %s", err)
		}

		newSQL, newArgs, err := s.buildStatsQuery(userUID, tknUUID, chain, rpcMethod, granularity)
		if err != nil {
			return nil, fmt.Errorf("buildStatsQuery 1: %s", err)
		}

		sqlQuery = fmt.Sprintf("SELECT * FROM (%s UNION ALL %s) AS combined ORDER BY ts", oldSQL, newSQL)
		args = append(args, oldArgs...)
		args = append(args, newArgs...)
	} else {
		sqlQuery, args, err = s.buildStatsQuery(userUID, tknUUID, chain, rpcMethod, granularity)
		if err != nil {
			return nil, fmt.Errorf("buildStatsQuery 2: %s", err)
		}
	}

	rows, err := s.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry ResponseTimeHistory
		if err = rows.Scan(&entry.RpcMethod, &entry.Network, &entry.Token, &entry.AvgResponseTimeMs, &entry.P95ResponseTimeMs, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		var defaultUUID uuid.UUID
		if entry.Token != nil && *entry.Token == defaultUUID {
			entry.Token = nil
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Storage) buildAggregatedQuery(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	granularity string,
	startTime time.Time,
) (string, []interface{}, error) {
	builder := sq.Select().PlaceholderFormat(sq.Question).OrderBy("ts")
	builder = buildWhereCondition(builder, userUID, tknUUID, chain, rpcMethod, false)
	table := userHourlyAggregatedTableName
	if granularity == DailyGranularity {
		table = userDailyAggregatedTableName
	}
	builder = builder.From(table)
	builder = builder.Columns(
		"coalesce(nullIf(rpc_method, ''), 'All')",
		"coalesce(nullIf(chain, ''), 'All')",
		"tkn_uuid",
		"avg_response_time_ms",
		"p95_response_time_ms",
	)
	newDataTimeEnd := calculateNewDataTimeEnd(granularity)
	if granularity == HourlyGranularity {
		builder = builder.Columns("timestamp as ts").GroupBy("timestamp").Where("timestamp >= ?", startTime).Where("timestamp < ?", newDataTimeEnd)
	} else {
		builder = builder.Columns("day as ts").GroupBy("day").Where("toDateTime(day) >= ?", startTime).Where("toDateTime(day) < ?", newDataTimeEnd)
	}
	builder = builder.GroupBy("avg_response_time_ms, p95_response_time_ms, rpc_method, chain, tkn_uuid")

	return builder.ToSql()
}

func (s *Storage) buildStatsQuery(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	granularity string,
) (string, []interface{}, error) {
	builder := sq.Select().PlaceholderFormat(sq.Question).OrderBy("ts")
	builder = buildWhereCondition(builder, userUID, tknUUID, chain, rpcMethod, true)
	builder = builder.From("aura.stats")

	if rpcMethod != nil {
		builder = builder.Columns("coalesce(nullIf(rpc_method, ''), 'All')").GroupBy("rpc_method")
	} else {
		builder = builder.Columns("'All' as rpc_method")
	}
	if chain != nil {
		builder = builder.Columns("coalesce(nullIf(chain, ''), 'All')").GroupBy("chain")
	} else {
		builder = builder.Columns("'All' as chain")
	}
	if tknUUID != nil {
		builder = builder.Columns("tkn_uuid").GroupBy("tkn_uuid")
	} else {
		builder = builder.Columns("toUUID('00000000-0000-0000-0000-000000000000') as tkn_uuid")
	}
	builder = builder.Columns(
		"toInt64(avg(response_time_ms)) as avg_response_time_ms",
		"toInt64(quantileTiming(0.95)(response_time_ms)) as p95_response_time_ms",
	)
	if granularity == HourlyGranularity {
		builder = builder.Columns("toStartOfHour(timestamp) as ts")
	} else {
		builder = builder.Columns("toDate(timestamp) as ts")
	}
	newDataTimeEnd := calculateNewDataTimeEnd(granularity)
	builder = builder.Where("timestamp >= ?", newDataTimeEnd).Where("timestamp <= now()")
	builder = builder.GroupBy("ts")

	return builder.ToSql()
}

func calculateNewDataTimeEnd(granularity string) time.Time {
	if granularity == HourlyGranularity {
		return time.Now().UTC().Truncate(hourlyThreshold).Add(-hourlyThreshold)
	}
	return time.Now().UTC().Truncate(dailyThreshold).Add(-dailyThreshold)
}
