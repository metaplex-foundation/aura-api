package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/internal/pkg/log"
	"github.com/adm-metaex/aura-api/internal/pkg/proto"
	"github.com/adm-metaex/aura-api/internal/pkg/util"
)

type StatsFilterCondition struct {
	Field string
	Value string
}

type Stat struct {
	Timestamp      time.Time `json:"timestamp" format:"date-time" example:"2023-10-05T21:51:25.913824Z"`
	UserUID        string    `json:"user_uid,omitempty"`
	RequestUUID    string    `json:"request_uuid" example:"20c022cd-16b0-4b92-8144-135583710fc4" format:"uuid"`
	Endpoint       string    `json:"endpoint" example:"http://127.0.0.1"`
	RPCErrorCode   string    `json:"rpc_error_code" example:"-32601"`
	UserAgent      string    `json:"user_agent,omitempty"`
	RPCMethod      string    `json:"rpc_method"`
	RPCRequestData string    `json:"rpc_request_data"`
	Chain          string    `json:"chain"`
	ExecutionTime  int64     `json:"execution_time_ms" example:"1"`
	ResponseTime   int64     `json:"response_time_ms" example:"1"`
	Status         uint16    `json:"status,omitempty" example:"200"`
	Attempts       uint8     `json:"attempts" example:"1"`
	Project        uuid.UUID `json:"prj_uuid,omitempty"`
}

const statsTableName = "stats"

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

func (s *Storage) DeleteOutdatedStats() error {
	query := `ALTER TABLE stats DELETE
    	WHERE toDate(now()) > toDate(timestamp);`

	_, err := s.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("exec: %s", err)
	}

	return nil
}

func (s *Storage) SelectStats(ctx context.Context, userUID string, project uuid.UUID, beforeReqUUID string, limit int, filters map[string][]string, sorts []StatsFilterCondition) (res []Stat, err error) {
	if userUID == "" {
		return nil, ErrEmptyUserUUID
	}

	q := sq.Select("request_uuid, status, execution_time_ms, endpoint, attempts, response_time_ms, rpc_error_code, user_agent, rpc_method, rpc_request_data, timestamp, chain").
		From(statsTableName).
		Where("user_uid = ? AND prj_uuid = ?", userUID, project)
	if limit != 0 {
		q = q.Limit(uint64(limit))
	}
	if beforeReqUUID != "" {
		q = q.Where("request_uuid < ?", beforeReqUUID)
	}

	for field, value := range filters {
		q = q.Where(fmt.Sprintf("%s IN (?)", field), value)
	}
	for _, sort := range sorts {
		q = q.OrderBy(fmt.Sprintf("%s %s", sort.Field, sort.Value))
	}

	if len(sorts) == 0 {
		q = q.OrderBy("request_uuid desc")
	}

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
		var r Stat
		err = rows.Scan(&r.RequestUUID, &r.Status, &r.ExecutionTime, &r.Endpoint, &r.Attempts, &r.ResponseTime, &r.RPCErrorCode,
			&r.UserAgent, &r.RPCMethod, &r.RPCRequestData, &r.Timestamp, &r.Chain)
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
