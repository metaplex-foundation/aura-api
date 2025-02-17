package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adm-metaex/aura-api/pkg/log"
)

type (
	ProviderRequestStats struct {
		Provider     string    `json:"provider"`
		Chain        string    `json:"chain"`
		RequestType  string    `json:"request_type"`
		RequestCount int64     `json:"request_count"`
		RequestPrice int64     `json:"request_price"`
		Day          time.Time `json:"day"`
	}

	// there is enum type in providers_requests_daily_summary ClickHouse table with two variants
	PaymentPlan int
)

const (
	// start from 1 like in ClickHouse providers_requests_daily_summary table
	PayAsYouGo PaymentPlan = iota + 1
	Subscription
)

var paymentPlanName = map[PaymentPlan]string{
	PayAsYouGo:   "pay-as-you-go",
	Subscription: "subscription",
}

func (pp PaymentPlan) String() string {
	return paymentPlanName[pp]
}

func (s *Storage) GetLatestAggregatedRewardsDayByPlan(ctx context.Context, paymentPlan PaymentPlan) (date Date, err error) {
	query := fmt.Sprintf(`SELECT toDate(MAX(time)) from aura.providers_requests_daily_summary WHERE payment_plan = '%s';`, paymentPlan.String())

	row := s.conn.QueryRow(query)

	var nullableDate sql.NullTime
	err = row.Scan(&nullableDate)
	if err != nil {
		return date, fmt.Errorf("scan: %s", err)
	}

	if !nullableDate.Valid {
		return Date{}, nil
	}

	return Date{nullableDate.Time}, nil
}

func (s *Storage) GetProviderRequestStatsPayAsYouGoPlan(ctx context.Context, startFromDay Date) (result []ProviderRequestStats, err error) {
	query := fmt.Sprintf(`
		SELECT
			provider,
			chain,
			request_type,
			COUNT(*) AS request_count,
			SUM(method_cost)/COUNT(*) as request_price,
			toDate(timestamp) AS day
		FROM aura.stats
			WHERE day > '%s'
			AND day < today()
			AND subscription_id = 2
			AND status = 200
		GROUP BY provider, chain, request_type, day
		ORDER BY day;
	`, startFromDay.Format("2006-01-02"))

	rows, err := s.conn.Query(query)
	if err != nil {
		return result, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry ProviderRequestStats
		if err = rows.Scan(&entry.Provider, &entry.Chain, &entry.RequestType, &entry.RequestCount, &entry.RequestPrice, &entry.Day); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Storage) GetProviderRequestStatsSubscriptionPlan(ctx context.Context, startFromDay Date) (result []ProviderRequestStats, err error) {
	query := fmt.Sprintf(`
		SELECT
			provider,
			chain,
			request_type,
			COUNT(*) AS request_count,
			method_cost as request_price,
			toDate(timestamp) AS day
		FROM aura.stats
			WHERE day > '%s'
			AND day < today()
			AND subscription_id >= 3
			AND status = 200
		GROUP BY provider, chain, request_type, day
		ORDER BY day;
	`, startFromDay.Format("2006-01-02"))

	rows, err := s.conn.Query(query)
	if err != nil {
		return result, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry ProviderRequestStats
		if err = rows.Scan(&entry.Provider, &entry.Chain, &entry.RequestType, &entry.RequestCount, &entry.RequestPrice, &entry.Day); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Storage) SaveAggregatedProvidersStat(ctx context.Context, aggregatedData []ProviderRequestStats, paymentPlan PaymentPlan) (err error) {
	if len(aggregatedData) == 0 {
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

	stmt, err := tx.Prepare(`INSERT INTO aura.providers_requests_daily_summary (
		provider,
		request_type,
		chain,
		payment_plan,
		price_per_request,
		num_of_requests,
		time
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	for _, stat := range aggregatedData {
		_, err := stmt.Exec(
			stat.Provider,
			stat.RequestType,
			stat.Chain,
			paymentPlan.String(),
			stat.RequestPrice,
			stat.RequestCount,
			stat.Day,
		)
		// TODO!: maybe convert Day with .format("...")
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
