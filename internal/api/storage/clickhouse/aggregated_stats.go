package clickhouse

import (
	"context"
	"fmt"
)

func (s *Storage) AggregateAnalysisStats() error {
	query := `INSERT INTO aggregated_analysis_data (chain, rpc_method, rpc_request_data, day, execution_time_ms, response_time_ms, total_req) 
        SELECT chain, rpc_method, rpc_request_data, toDate(timestamp) as day, avg(execution_time_ms), avg(response_time_ms), count(rpc_method) as c
		FROM stats
		WHERE day < toDate(now(), 'Etc/UTC')
		GROUP BY chain, rpc_method, rpc_request_data, day
		ORDER BY chain, rpc_method, c desc
		LIMIT 100 BY rpc_method, day`
	_, err := s.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	_, err = s.conn.Exec(`OPTIMIZE TABLE aggregated_analysis_data FINAL`)
	if err != nil {
		return fmt.Errorf("optimize: %s", err)
	}

	return nil
}

func (s *Storage) AggregateAnalysisData(ctx context.Context, aggregateOnlyRecentData bool) error {
	// TODO: use query builder
	selectRecentDataCondition := ""
	if aggregateOnlyRecentData {
		selectRecentDataCondition = "AND toDate(timestamp) >= toDate(now() - INTERVAL 2 DAY, 'Etc/UTC')"
	}
	query := fmt.Sprintf(`
   	INSERT INTO aura.aggregated_analysis_data
	SELECT
	    chain,
	    rpc_method,
	    rpc_request_data,
	    toDate(timestamp) as day,
	    avg(execution_time_ms),
	    avg(response_time_ms),
	    quantileTiming(0.95)(response_time_ms) AS p95_response_time_ms,
	    count(rpc_method) as total_req
	FROM aura.stats
	WHERE day < toDate(now(), 'Etc/UTC') %s
	GROUP BY chain, rpc_method, rpc_request_data, day
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
