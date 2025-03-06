package clickhouse

import (
	"context"
	"fmt"
	"math/rand"
	"time"

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

type Count struct {
	int64
}

const (
	userDailyAggregatedTableName  = "aura.aggregated_user_daily_data"
	userHourlyAggregatedTableName = "aura.aggregated_user_hourly_data"
)

func (s *Storage) InsertMockData(count int) error {
	ctx := context.Background()

	userUID := "1e05920a-bf02-45bf-96a1-a2062c2e0056"
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
		target_type,
	    is_mainnet
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

	now := time.Now()
	startTime := now.Add(-15 * 24 * time.Hour)

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
		diff := rand.Int63n(int64(15 * 24 * time.Hour))
		ts := startTime.Add(time.Duration(diff))

		// response_size_bytes
		responseSize := int64(rand.Intn(10001) + 100)

		// endpoint, user_agent, server_id, rpc_request_data, target_type - на ваш розсуд
		endpoint := "/api/" + chain
		userAgent := "Go-http-client/1.1"
		serverID := "server-1"
		rpcRequestData := "{\"param\":\"value\"}"
		targetType := "rpc"

		var isMainnet bool
		if rand.Intn(2) == 0 {
			isMainnet = true
		}
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
			isMainnet,
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

func (s *Storage) InsertMockedData(count int) error {
	ctx := context.Background()
	userUID := "1e05920a-bf02-45bf-96a1-a2062c2e0056"
	tknUUIDs := []uuid.UUID{
		uuid.New(),
		//uuid.New(),
		uuid.New(),
	}

	// Підготуємо INSERT запит
	// Вставляємо дані пачкою (batch insert) для оптимізації
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO aura.user_subscription_usage (time, chain, user_uid, tkn_uuid, used_credits, is_mainnet) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Поточний час
	now := time.Now()
	// Часова межа 20 днів назад
	twentyDaysAgo := now.Add(-20 * 24 * time.Hour)

	chains := []string{"solana", "aura"}

	// Заповнення випадковими даними
	for i := 0; i < count; i++ {
		// Генеруємо випадковий час між twentyDaysAgo та now
		diff := now.Sub(twentyDaysAgo)
		randDuration := time.Duration(rand.Int63n(diff.Nanoseconds()))
		randomTime := twentyDaysAgo.Add(randDuration)

		// Випадковий chain
		chain := chains[rand.Intn(len(chains))]

		// Випадковий tkn_uuid з трьох
		chosenTkn := tknUUIDs[rand.Intn(len(tknUUIDs))]

		// Випадковий used_credits від 0 до 100
		usedCredits := rand.Int63n(101) // [0,100]
		var isMainnet bool
		if rand.Intn(2) == 0 {
			isMainnet = true
		}
		_, err := stmt.ExecContext(ctx, randomTime, chain, userUID, chosenTkn, usedCredits, isMainnet)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert row: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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
		coalesce(nullIf(request_type, ''), 'All'),
	    count(*) AS total_req,
	    countIf(rpc_error_code = '0' AND status = 200) AS success_req,
	    countIf(status != 200) AS http_err,
	    countIf(rpc_error_code != '0') AS rpc_err,
	    sum(response_size_bytes) AS response_size_bytes,
	    avg(response_time_ms) AS avg_response_time_ms,
	    quantileTiming(0.95)(response_time_ms) AS p95_response_time_ms,
	    is_mainnet
	FROM aura.stats
	WHERE timestamp < date_trunc('hour', now()) %s
	GROUP BY GROUPING SETS (
		    (user_uid, tkn_uuid, toDateTime(toStartOfHour(timestamp)), rpc_method, chain, request_type, is_mainnet),
		    (user_uid, tkn_uuid, toDateTime(toStartOfHour(timestamp)), chain, request_type, is_mainnet),
		    (user_uid, tkn_uuid, toDateTime(toStartOfHour(timestamp)), is_mainnet),
		    (user_uid, toDateTime(toStartOfHour(timestamp)), rpc_method, chain, request_type, is_mainnet),
		    (user_uid, toDateTime(toStartOfHour(timestamp)), chain, request_type, is_mainnet),
		    (user_uid, toDateTime(toStartOfHour(timestamp)), is_mainnet)
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
		coalesce(nullIf(request_type, ''), 'All'),
	    count(*) AS total_req,
	    countIf(rpc_error_code = '0' AND status = 200) AS success_req,
	    countIf(status != 200) AS http_err,
	    countIf(rpc_error_code != '0') AS rpc_err,
	    sum(response_size_bytes) AS response_size_bytes,
	    avg(response_time_ms) AS avg_response_time_ms,
	    quantileTiming(0.95)(response_time_ms) AS p95_response_time_ms,
		is_mainnet
	FROM aura.stats
	WHERE toDate(timestamp) < toDate(now(), 'Etc/UTC') %s
	GROUP BY GROUPING SETS (
	    (user_uid, tkn_uuid, toDate(timestamp), rpc_method, chain, request_type, is_mainnet),
		(user_uid, tkn_uuid, toDate(timestamp), chain, request_type, is_mainnet),
		(user_uid, tkn_uuid, toDate(timestamp), is_mainnet),
		(user_uid, toDate(timestamp), rpc_method, chain, request_type, is_mainnet),
		(user_uid, toDate(timestamp), chain, request_type, is_mainnet),
		(user_uid, toDate(timestamp), is_mainnet)
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

func (s *Storage) GetNumberOfActiveUsers(ctx context.Context) (result int64, err error) {
	query := `SELECT COUNT(DISTINCT user_uid) as count FROM aura.aggregated_user_daily_data;`

	var count Count

	row := s.conn.QueryRow(query)
	err = row.Scan(&count)
	if err != nil {
		return result, fmt.Errorf("scan: %s", err)
	}

	return count.int64, nil
}
