package clickhouse

import (
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
