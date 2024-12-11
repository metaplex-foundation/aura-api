package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/pkg/log"
)

const userSubscriptionUsageTable = "aura.user_subscription_usage"

type UserSubscriptionUsage struct {
	Date  Date  `json:"date" swaggertype:"string" format:"date"`
	Value int64 `json:"value"`
}

type (
	CreditsUsageHistory struct {
		Timestamp   time.Time `json:"timestamp"`
		CreditsUsed int64     `json:"credits_used"`
		Network     string    `json:"network"`
	}
)

func (r CreditsUsageHistory) GetTimestamp() time.Time {
	return r.Timestamp
}

func (r CreditsUsageHistory) BuildDefault(rpcMethod, network *string, token *uuid.UUID, t time.Time) CreditsUsageHistory {
	if network == nil {
		r.Network = allData
	} else {
		r.Network = *network
	}
	r.Timestamp = t

	return r
}

func (s *Storage) BatchInsertUserSubscriptionUsage(reqs map[string]int64) error {
	if len(reqs) == 0 {
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

	stmt, err := tx.Prepare(`INSERT INTO user_subscription_usage (
		time,
		user_uid,
        used_credits
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	timeNow := time.Now()
	for userUID, usedCredits := range reqs {
		_, err = stmt.Exec(
			timeNow,
			userUID,
			usedCredits,
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

func (s *Storage) SelectUserSubscriptionUsage(ctx context.Context, userUID string, start, end time.Time) (res []UserSubscriptionUsage, err error) {
	if userUID == "" {
		return nil, ErrEmptyUserUUID
	}

	q := `SELECT toDate(time) AS date, sum(used_credits) FROM user_subscription_usage
		WHERE time >= ? AND time < ? AND user_uid = ?
		GROUP BY date, user_uid
		ORDER BY date`

	rows, err := s.conn.QueryContext(ctx, q, start, end, userUID)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		var r UserSubscriptionUsage
		err = rows.Scan(&r.Date.Time, &r.Value)
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

func (s *Storage) GetCreditsUsageHistory(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
) (result []CreditsUsageHistory, err error) {
	if userUID == "" {
		return result, ErrEmptyUserUUID
	}
	builder := sq.Select().
		PlaceholderFormat(sq.Question).
		Columns("chain", "sum(used_credits) as used_credits").
		From(userSubscriptionUsageTable).
		OrderBy("ts").
		GroupBy("ts, chain").
		Where("time >= ?", startTime)
	if granularity == HourlyGranularity {
		builder = builder.Columns("toStartOfHour(time) as ts")
	} else {
		builder = builder.Columns("toDateTime(toDate(time)) as ts")
	}
	builder = buildWhereCondition(builder, userUID, tknUUID, chain, rpcMethod, true)

	sqlQuery, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ToSql: %s", err)
	}

	rows, err := s.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry CreditsUsageHistory
		if err = rows.Scan(&entry.Network, &entry.CreditsUsed, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	return fillGaps(result, startTime, granularity, rpcMethod, chain, tknUUID), nil
}
