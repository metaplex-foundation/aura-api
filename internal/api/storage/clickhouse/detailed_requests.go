package clickhouse

import (
	"database/sql"
	"fmt"

	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/proto"
)

func (s *Storage) BatchInsertDetailedRequests(stats []*proto.DetailedRequest) error {
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

	stmt, err := tx.Prepare(`INSERT INTO detailed_requests (
		user_uid,
        request_uuid,
        status,
        execution_time_ms,
        endpoint,
        attempts,
        rpc_error_code,
        user_agent,
        rpc_method,
        rpc_request_body,
        timestamp,
        server_id,
        chain,
        response_size_bytes
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	for _, stat := range stats {
		_, err = stmt.Exec(
			stat.GetUserUid(),
			stat.GetRequestUuid(),
			uint16(stat.GetStatus()),
			stat.GetExecutionTimeMs(),
			stat.GetEndpoint(),
			uint8(stat.GetAttempts()),
			stat.GetRpcErrorCode(),
			stat.GetUserAgent(),
			stat.GetRpcMethod(),
			stat.GetRpcRequestBody(),
			stat.GetTimestamp().AsTime(),
			s.serverID,
			stat.GetChain(),
			stat.GetResponseSizeBytes(),
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
