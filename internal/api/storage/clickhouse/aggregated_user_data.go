package clickhouse

import (
	"context"
	"errors"
	"fmt"

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

const userAggregatedTableName = "aggregated_user_data"

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
		From(userAggregatedTableName).
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
