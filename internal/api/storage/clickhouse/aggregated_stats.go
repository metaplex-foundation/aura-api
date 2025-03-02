package clickhouse

import (
	"context"
	"fmt"
)

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
