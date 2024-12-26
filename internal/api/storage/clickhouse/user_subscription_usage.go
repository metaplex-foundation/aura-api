package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/pkg/log"
	auraProto "github.com/adm-metaex/aura-api/pkg/proto"
)

const userSubscriptionUsageTable = "aura.user_subscription_usage"

type UserSubscriptionUsage struct {
	Date  Date  `json:"date" swaggertype:"string" format:"date"`
	Value int64 `json:"value"`
}

type (
	CreditsUsageHistory struct {
		Timestamp time.Time        `json:"timestamp"`
		Networks  map[string]int64 `json:"networks"`
	}
)

func (r CreditsUsageHistory) GetTimestamp() time.Time {
	return r.Timestamp
}

func (r CreditsUsageHistory) BuildDefault(rpcMethod, network *string, token *uuid.UUID, t time.Time) CreditsUsageHistory {
	r.Timestamp = t
	r.Networks = map[string]int64{
		"eclipse":            0,
		"solana":             0,
		"getProgramAccounts": 0,
	}

	return r
}

func (s *Storage) BatchInsertUserSubscriptionUsage(reqs map[string]*auraProto.UserRequestsByChain) error {
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
        used_credits,
        chain,
        tkn_uuid                             
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	timeNow := time.Now()
	for userUID, reqChain := range reqs {
		for chain, reqToken := range reqChain.GetReqs() {
			for token, usedCredits := range reqToken.GetReqs() {
				_, err = stmt.Exec(
					timeNow,
					userUID,
					usedCredits,
					chain,
					token,
				)
				if err != nil {
					return fmt.Errorf("exec statement error: %s", err)
				}
			}
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
	if granularity == HourlyGranularity {
		startTime = startTime.Truncate(time.Hour)
	} else {
		startTime = startTime.Truncate(24 * time.Hour)
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
	sqlQuery = fmt.Sprintf(`SELECT
    	mapFromArrays(
    	        groupArray(chain),
    	        groupArray(used_credits)
    	    ) AS chain_map,
    	ts
		FROM (%s)
		GROUP BY ts
		ORDER BY ts`, sqlQuery)

	rows, err := s.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry CreditsUsageHistory
		if err = rows.Scan(&entry.Networks, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	// TODO: refactor
	for i := range result {
		for _, network := range []string{"solana", "eclipse", "getProgramAccounts"} {
			if _, ok := result[i].Networks[network]; !ok {
				result[i].Networks[network] = 0
			}
		}
	}
	return fillGaps(result, startTime, granularity, rpcMethod, chain, tknUUID), nil
}
