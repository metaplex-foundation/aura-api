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
		AvgResponseTimeMs *int64     `json:"avg_response_time_ms"`
		P95ResponseTimeMs *int64     `json:"p95_response_time_ms"`
		RpcMethod         string     `json:"rpc_method"`
		Network           string     `json:"network"`
		Token             *uuid.UUID `json:"token,omitempty"`
	}
	RequestsVolumeHistory struct {
		Timestamp     time.Time  `json:"timestamp"`
		TotalRequests int64      `json:"total_requests"`
		TotalErrors   int64      `json:"total_errors"`
		RpcMethod     string     `json:"rpc_method"`
		Network       string     `json:"network"`
		Token         *uuid.UUID `json:"token,omitempty"`
	}
)

type TimeSeriesEntry[T any] interface {
	GetTimestamp() time.Time
	BuildDefault(rpcMethod, network *string, token *uuid.UUID, t time.Time) T
}

func (r RequestsVolumeHistory) GetTimestamp() time.Time {
	return r.Timestamp
}

func (r RequestsVolumeHistory) BuildDefault(rpcMethod, network *string, token *uuid.UUID, t time.Time) RequestsVolumeHistory {
	if rpcMethod == nil {
		r.RpcMethod = allData
	} else {
		r.RpcMethod = *rpcMethod
	}
	if network == nil {
		r.Network = allData
	} else {
		r.Network = *network
	}
	r.Token = token
	r.Timestamp = t

	return r
}

func (r ResponseTimeHistory) GetTimestamp() time.Time {
	return r.Timestamp
}

func (r ResponseTimeHistory) BuildDefault(rpcMethod, network *string, token *uuid.UUID, t time.Time) ResponseTimeHistory {
	if rpcMethod == nil {
		r.RpcMethod = allData
	} else {
		r.RpcMethod = *rpcMethod
	}
	if network == nil {
		r.Network = allData
	} else {
		r.Network = *network
	}
	r.Token = token
	r.Timestamp = t

	return r
}

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
        chain,
        response_size_bytes,
        target_type,
        is_mainnet
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	for _, stat := range stats {
		_, err = stmt.Exec(
			stat.GetUserUid(),
			util.ParseUUIDOrDefault(stat.GetTokenUuid()),
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
			stat.GetProvider(),
			stat.GetMethodCost(),
			stat.GetChain(),
			stat.GetResponseSizeBytes(),
			stat.GetTargetType(),
			stat.GetIsMainnet(),
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

func buildWhereCondition(builder sq.SelectBuilder, userUID string, tknUUID *uuid.UUID, chain *string, rpcMethod *string, isFromStats bool, isMainnet *bool) sq.SelectBuilder {
	builder = builder.Where(sq.Eq{"user_uid": userUID})
	if isMainnet != nil || !isFromStats {
		builder = builder.Where(sq.Eq{"is_mainnet": isMainnet}).GroupBy("is_mainnet")
	}

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

func (s *Storage) prepareHistoryQuery(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
	aggregatedColumns []string,
	statsColumns []string,
	isMainnet *bool,
) (sqlQuery string, args []interface{}, err error) {
	if userUID == "" {
		return sqlQuery, args, ErrEmptyUserUUID
	}
	diff := time.Since(startTime)
	isAggregated := (diff > dailyThreshold && granularity == DailyGranularity) || (diff > hourlyThreshold && granularity == HourlyGranularity)

	if isAggregated {
		oldSQL, oldArgs, err := s.buildAggregatedQuery(userUID, tknUUID, chain, rpcMethod, granularity, startTime, aggregatedColumns, isMainnet)
		if err != nil {
			return sqlQuery, args, fmt.Errorf("buildAggregatedQuery: %s", err)
		}

		newSQL, newArgs, err := s.buildStatsQuery(
			userUID,
			tknUUID,
			chain,
			rpcMethod,
			granularity,
			statsColumns,
			isMainnet,
		)
		if err != nil {
			return sqlQuery, args, fmt.Errorf("buildStatsQuery (aggregated case): %s", err)
		}

		sqlQuery = fmt.Sprintf("SELECT * FROM (%s UNION ALL %s) AS combined ORDER BY ts", oldSQL, newSQL)
		args = append(args, oldArgs...)
		args = append(args, newArgs...)
	} else {
		sqlQuery, args, err = s.buildStatsQuery(
			userUID,
			tknUUID,
			chain,
			rpcMethod,
			granularity,
			statsColumns,
			isMainnet,
		)
		if err != nil {
			return sqlQuery, args, fmt.Errorf("buildStatsQuery (non-aggregated): %s", err)
		}
	}

	return sqlQuery, args, nil
}

func (s *Storage) GetRequestsVolumeHistory(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
	isMainnet *bool,
) (result []RequestsVolumeHistory, err error) {
	sqlQuery, args, err := s.prepareHistoryQuery(
		userUID,
		tknUUID,
		chain,
		rpcMethod,
		startTime,
		granularity,
		[]string{"total_req", "http_err + rpc_err AS total_err"},                                                   // aggregatedColumns
		[]string{"count(*) AS total_req", "countIf(status != 200) +  countIf(rpc_error_code != '0') AS total_err"}, // statsColumns
		isMainnet,
	)
	if err != nil {
		return nil, fmt.Errorf("prepareHistoryQuery: %s", err)
	}

	rows, err := s.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry RequestsVolumeHistory
		if err = rows.Scan(&entry.RpcMethod, &entry.Network, &entry.Token, &entry.TotalRequests, &entry.TotalErrors, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		var defaultUUID uuid.UUID
		if entry.Token != nil && *entry.Token == defaultUUID {
			entry.Token = nil
		}
		result = append(result, entry)
	}
	return fillGaps(result, startTime, granularity, rpcMethod, chain, tknUUID), nil
}

func (s *Storage) GetResponseTimeHistory(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
	isMainnet *bool,
) (result []ResponseTimeHistory, err error) {
	sqlQuery, args, err := s.prepareHistoryQuery(
		userUID,
		tknUUID,
		chain,
		rpcMethod,
		startTime,
		granularity,
		[]string{"avg_response_time_ms", "p95_response_time_ms"}, // aggregatedColumns
		[]string{
			"toInt64(avg(response_time_ms)) as avg_response_time_ms",
			"toInt64(quantileTiming(0.95)(response_time_ms)) as p95_response_time_ms",
		}, // statsColumns
		isMainnet,
	)
	if err != nil {
		return nil, fmt.Errorf("prepareHistoryQuery: %s", err)
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
	return fillGaps(result, startTime, granularity, rpcMethod, chain, tknUUID), nil
}

func (s *Storage) buildAggregatedQuery(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	granularity string,
	startTime time.Time,
	columnsToSelect []string,
	isMainnet *bool,
) (string, []interface{}, error) {
	builder := sq.Select().PlaceholderFormat(sq.Question).OrderBy("ts")
	builder = buildWhereCondition(builder, userUID, tknUUID, chain, rpcMethod, false, isMainnet)
	table := userHourlyAggregatedTableName
	if granularity == DailyGranularity {
		table = userDailyAggregatedTableName
	}
	builder = builder.From(table)
	builder = builder.Columns(
		"coalesce(nullIf(rpc_method, ''), 'All')",
		"coalesce(nullIf(chain, ''), 'All')",
		"tkn_uuid",
	)
	for _, column := range columnsToSelect {
		builder = builder.Columns(column).GroupBy(column)
	}
	newDataTimeEnd := calculateNewDataTimeEnd(granularity)
	if granularity == HourlyGranularity {
		builder = builder.Columns("timestamp as ts").GroupBy("timestamp").Where("timestamp >= ?", startTime).Where("timestamp < ?", newDataTimeEnd)
	} else {
		builder = builder.Columns("day as ts").GroupBy("day").Where("toDateTime(day) >= ?", startTime).Where("toDateTime(day) < ?", newDataTimeEnd)
	}
	builder = builder.GroupBy("rpc_method, chain, tkn_uuid")

	return builder.ToSql()
}

func (s *Storage) buildStatsQuery(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	granularity string,
	columnsToSelect []string,
	isMainnet *bool,
) (string, []interface{}, error) {
	builder := sq.Select().PlaceholderFormat(sq.Question).OrderBy("ts")
	builder = buildWhereCondition(builder, userUID, tknUUID, chain, rpcMethod, true, isMainnet)
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
	for _, column := range columnsToSelect {
		builder = builder.Columns(column)
	}
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

func incrementByGranularity(t time.Time, granularity string) time.Time {
	if granularity == HourlyGranularity {
		return t.Add(time.Hour)
	} else {
		return t.AddDate(0, 0, 1)
	}
}

func fillGapsBetween[T TimeSeriesEntry[T]](base []T, start, end time.Time, granularity string, rpcMethod, network *string, token *uuid.UUID) []T {
	if base == nil {
		base = []T{}
	}

	t := incrementByGranularity(start, granularity)
	for t.Before(end) && !t.Equal(end) {
		var gap T
		base = append(base, gap.BuildDefault(rpcMethod, network, token, t))
		t = incrementByGranularity(t, granularity)
	}

	return base
}

func fillGaps[T TimeSeriesEntry[T]](entries []T, startTime time.Time, granularity string, rpcMethod, network *string, token *uuid.UUID) []T {
	if len(entries) == 0 {
		return entries
	}

	finalResult := make([]T, 0, len(entries)*2)
	now := time.Now()
	if granularity == HourlyGranularity {
		startTime = startTime.Truncate(time.Hour)
	} else {
		startTime = startTime.Truncate(24 * time.Hour)
	}

	firstTimestamp := entries[0].GetTimestamp()
	if firstTimestamp.After(startTime) {
		finalResult = fillGapsBetween(finalResult, startTime, firstTimestamp, granularity, rpcMethod, network, token)
	}

	finalResult = append(finalResult, entries[0])
	lastTimestamp := entries[0].GetTimestamp()

	for i := 1; i < len(entries); i++ {
		nextTimestamp := entries[i].GetTimestamp()
		if nextTimestamp.After(lastTimestamp) {
			finalResult = fillGapsBetween(finalResult, lastTimestamp, nextTimestamp, granularity, rpcMethod, network, token)
		}
		finalResult = append(finalResult, entries[i])
		lastTimestamp = entries[i].GetTimestamp()
	}

	if lastTimestamp.Before(now) {
		finalResult = fillGapsBetween(finalResult, lastTimestamp, now, granularity, rpcMethod, network, token)
	}

	return finalResult
}
