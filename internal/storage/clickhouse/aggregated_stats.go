package clickhouse

import (
	"context"
	"fmt"
	"time"
)

type RequestsByChainAndType struct {
	Day         time.Time `json:"day"`
	Chain       string    `json:"chain"`
	RequestType string    `json:"request_type"`
	Requests    int64     `json:"requests"`
}

func (s *Storage) AggregateAnalysisData(ctx context.Context, aggregateOnlyRecentData bool) error {
	// TODO: use query builder
	var selectRecentDataCondition string
	if aggregateOnlyRecentData {
		selectRecentDataCondition = "AND toDate(timestamp) >= toDate(now() - INTERVAL 2 DAY, 'Etc/UTC')"
	}
	query := fmt.Sprintf(`
   	INSERT INTO aura.aggregated_analysis_data
	SELECT
	    chain,
		request_type,
	    rpc_method,
	    rpc_request_data,
	    provider,
	    toDate(timestamp) as day,
	    avg(execution_time_ms),
	    avg(response_time_ms),
	    quantileTiming(0.95)(response_time_ms) AS p95_response_time_ms,
	    count(rpc_method) as total_req,
	    is_mainnet
	FROM aura.stats
	WHERE day < toDate(now(), 'Etc/UTC') %s
	GROUP BY chain, request_type, is_mainnet, provider, rpc_method, rpc_request_data, day
    `, selectRecentDataCondition)
	_, err := s.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("exec 1: %s", err)
	}

	_, err = s.conn.ExecContext(ctx, `OPTIMIZE TABLE aura.aggregated_analysis_data FINAL`)
	if err != nil {
		return fmt.Errorf("optimize: %s", err)
	}

	return nil
}

func (s *Storage) GetDailyRequestsByChainAndType(ctx context.Context, startDate time.Time, endDate time.Time) (result []RequestsByChainAndType, err error) {
	query := fmt.Sprintf(`
		SELECT
			day,
			chain,
			request_type,
			SUM(total_req) AS requests
		FROM
			aura.aggregated_analysis_data
		WHERE
			day >= '%s'
			AND day <= '%s'
		GROUP BY
			chain,
			request_type,
			day
		ORDER BY day;
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	rows, err := s.conn.Query(query)
	if err != nil {
		return result, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry RequestsByChainAndType
		if err = rows.Scan(&entry.Day, &entry.Chain, &entry.RequestType, &entry.Requests); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	return result, nil
}
