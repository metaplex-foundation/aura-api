package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

type UserAggregatedStat struct {
	Date              Date      `json:"date" swaggertype:"string" format:"date"`
	UserUID           string    `json:"user_uid,omitempty" swaggerignore:"true"`
	Chain             string    `json:"chain" example:"solana"`
	RPCMethod         string    `json:"rpc_method"`
	TotalRequests     int64     `json:"total_requests" example:"200"`
	SuccessRequests   int64     `json:"success_requests" example:"1"`
	HTTPErrors        int64     `json:"http_errors" example:"1"`
	RPCErrors         int64     `json:"rpc_errors" example:"1"`
	Project           uuid.UUID `json:"prj_uuid,omitempty" swaggerignore:"true"`
	ResponseSizeBytes int64     `json:"response_size_bytes"`
}

const (
	userDailyAggregatedTableName  = "aura.aggregated_user_daily_data"
	userHourlyAggregatedTableName = "aura.aggregated_user_hourly_data"
)

func (s *Storage) InsertMockData(count int) error {
	ctx := context.Background()

	userUID := "user_123"
	tknUUIDs := []uuid.UUID{
		uuid.New(),
		//uuid.New(),
		uuid.New(),
	}

	solanaMethods := []string{
		"getAccountInfo", "getBalance", "getBlockTime", "getTransaction",
	}

	auraMethods := []string{
		"getAsset", "getAssetProof", "searchAssets",
	}

	chains := []string{"solana", "aura"}

	query := `
	INSERT INTO aura.stats (
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
		chain,
		response_size_bytes,
		target_type
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Add(-1 * 24 * time.Hour)
	startTime := now.Add(-3 * 24 * time.Hour)

	for i := 0; i < count; i++ {
		tkn := tknUUIDs[rand.Intn(len(tknUUIDs))]
		chain := chains[rand.Intn(len(chains))]
		var rpcMethod string
		if chain == "solana" {
			rpcMethod = solanaMethods[rand.Intn(len(solanaMethods))]
		} else {
			rpcMethod = auraMethods[rand.Intn(len(auraMethods))]
		}
		responseTime := int64(rand.Intn(10001))  // 0 до 30000
		executionTime := int64(rand.Intn(10001)) // 0 до 30000

		// attempts здебільшого 1
		attempts := uint8(1)
		if rand.Float64() < 0.1 { // 10% випадків буде більше 1
			attempts = uint8(rand.Intn(5) + 1)
		}

		// status 98% = 200, 2% != 200
		var status uint16
		if rand.Float64() < 0.98 {
			status = 200
		} else {
			// Нехай будуть рандомні 400 чи 500
			if rand.Intn(2) == 0 {
				status = 400 + uint16(rand.Intn(100))
			} else {
				status = 500 + uint16(rand.Intn(100))
			}
		}

		// rpc_error_code = '0' якщо status=200, інакше щось інше
		rpcErrorCode := "0"
		if status != 200 {
			rpcErrorCode = fmt.Sprintf("%d", status)
		}

		// timestamp - рандомний у останніх 30 днях
		diff := rand.Int63n(int64(3 * 24 * time.Hour))
		ts := startTime.Add(time.Duration(diff))

		// response_size_bytes
		responseSize := int64(rand.Intn(10001) + 100)

		// endpoint, user_agent, server_id, rpc_request_data, target_type - на ваш розсуд
		endpoint := "/api/" + chain
		userAgent := "Go-http-client/1.1"
		serverID := "server-1"
		rpcRequestData := "{\"param\":\"value\"}"
		targetType := "rpc"

		_, err = stmt.ExecContext(ctx,
			userUID,
			tkn,
			uuid.New(), // request_uuid унікальний
			status,
			executionTime,
			endpoint,
			attempts,
			responseTime,
			rpcErrorCode,
			userAgent,
			rpcMethod,
			rpcRequestData,
			ts,
			serverID,
			chain,
			responseSize,
			targetType,
		)
		if err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Storage) AggregateUserDataHourly(ctx context.Context, aggregateOnlyRecentData bool) error {
	// TODO: use query builder
	selectRecentDataCondition := ""
	if aggregateOnlyRecentData {
		selectRecentDataCondition = "AND timestamp >= date_trunc('hour', now() - INTERVAL 2 HOUR)"
	}
	query := fmt.Sprintf(`
	INSERT INTO aura.aggregated_user_hourly_data
	SELECT
	    user_uid,
	    tkn_uuid,
	    toDateTime(toStartOfHour(timestamp)),
	    coalesce(nullIf(rpc_method, ''), 'All'),
	    coalesce(nullIf(chain, ''), 'All'),
	    count(*) AS total_req,
	    countIf(rpc_error_code = '0' AND status = 200) AS success_req,
	    countIf(status != 200) AS http_err,
	    countIf(rpc_error_code != '0') AS rpc_err,
	    sum(response_size_bytes) AS response_size_bytes,
	    avg(response_time_ms) AS avg_response_time_ms,
	    quantileTiming(0.95)(response_time_ms) AS p95_response_time_ms
	FROM aura.stats
	WHERE timestamp < date_trunc('hour', now()) %s
	GROUP BY GROUPING SETS (
		    (user_uid, tkn_uuid, toDateTime(toStartOfHour(timestamp)), rpc_method, chain),
		    (user_uid, tkn_uuid, toDateTime(toStartOfHour(timestamp)), chain),
		    (user_uid, tkn_uuid, toDateTime(toStartOfHour(timestamp))),
		    (user_uid, toDateTime(toStartOfHour(timestamp)), rpc_method, chain),
		    (user_uid, toDateTime(toStartOfHour(timestamp)), chain),
		    (user_uid, toDateTime(toStartOfHour(timestamp)))
	)
	`, selectRecentDataCondition)

	_, err := s.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	_, err = s.conn.ExecContext(ctx, `OPTIMIZE TABLE aura.aggregated_user_hourly_data FINAL`)
	if err != nil {
		return fmt.Errorf("optimize: %s", err)
	}

	return nil
}

func (s *Storage) AggregateUserDataDaily(ctx context.Context, aggregateOnlyRecentData bool) error {
	// TODO: use query builder
	selectRecentDataCondition := ""
	if aggregateOnlyRecentData {
		selectRecentDataCondition = "AND toDate(timestamp) >= toDate(now() - INTERVAL 2 DAY, 'Etc/UTC')"
	}
	query := fmt.Sprintf(`
	INSERT INTO aura.aggregated_user_daily_data
	SELECT
	    user_uid,
	    tkn_uuid,
	    toDate(timestamp) AS day,
	    coalesce(nullIf(rpc_method, ''), 'All'),
	    coalesce(nullIf(chain, ''), 'All'),
	    count(*) AS total_req,
	    countIf(rpc_error_code = '0' AND status = 200) AS success_req,
	    countIf(status != 200) AS http_err,
	    countIf(rpc_error_code != '0') AS rpc_err,
	    sum(response_size_bytes) AS response_size_bytes,
	    avg(response_time_ms) AS avg_response_time_ms,
	    quantileTiming(0.95)(response_time_ms) AS p95_response_time_ms
	FROM aura.stats
	WHERE toDate(timestamp) < toDate(now(), 'Etc/UTC') %s
	GROUP BY GROUPING SETS (
	    (user_uid, tkn_uuid, toDate(timestamp), rpc_method, chain),
	    (user_uid, tkn_uuid, toDate(timestamp), chain),
	    (user_uid, tkn_uuid, toDate(timestamp)),
	    (user_uid, toDate(timestamp), rpc_method, chain),
	    (user_uid, toDate(timestamp), chain),
	    (user_uid, toDate(timestamp))
	)
    `, selectRecentDataCondition)
	_, err := s.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	_, err = s.conn.ExecContext(ctx, `OPTIMIZE TABLE aura.aggregated_user_daily_data FINAL`)
	if err != nil {
		return fmt.Errorf("optimize: %s", err)
	}

	return nil
}

func (s *Storage) AggregateUserData() error {
	query := `INSERT INTO aggregated_user_data (user_uid, prj_uuid, day, rpc_method, chain, total_req, success_req, http_err, rpc_err, response_size_bytes) 
	SELECT  user_uid,
			prj_uuid,
			toDate(timestamp) as day,
			rpc_method,
			chain,
			count(rpc_method) as c,
			count(if(rpc_error_code == '0' AND status == 200 AND rpc_method != '', true, null)),
			count(if(status != 200, true, null)),
			count(if(rpc_error_code != '0', true, null)),
			sum(response_size_bytes)
	FROM stats
	WHERE day < toDate(now(), 'Etc/UTC')
	GROUP BY user_uid, prj_uuid, day, rpc_method, chain`
	_, err := s.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	_, err = s.conn.Exec(`OPTIMIZE TABLE aggregated_user_data FINAL`)
	if err != nil {
		return fmt.Errorf("optimize: %s", err)
	}

	return nil
}

func (s *Storage) SelectUserAggregatedStats(ctx context.Context, userUID string, project uuid.UUID, interval int) (res []UserAggregatedStat, err error) {
	if userUID == "" {
		return nil, ErrEmptyUserUUID
	}
	if interval <= 0 {
		return nil, errors.New("invalid interval")
	}

	q := sq.Select("chain, rpc_method, total_req, success_req, http_err, rpc_err, day, response_size_bytes").
		From(userHourlyAggregatedTableName).
		Where("user_uid = ? AND prj_uuid = ? AND day >= toDate(now()) - interval ? day", userUID, project, interval).
		OrderBy("day desc")

	query, args, err := q.ToSql()
	if err != nil {
		return res, err
	}

	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		var r UserAggregatedStat
		err := rows.Scan(&r.Chain, &r.RPCMethod, &r.TotalRequests, &r.SuccessRequests, &r.HTTPErrors, &r.RPCErrors, &r.Date.Time, &r.ResponseSizeBytes)
		if err != nil {
			return res, err
		}

		res = append(res, r)
	}

	if err := rows.Err(); err != nil {
		return res, err
	}

	return
}
