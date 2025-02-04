package clickhouse

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/adm-metaex/aura-api/pkg/log"
)

type (
	ResponseProviderDailyRequests struct {
		Provider    string    `json:"provider"`
		Chain       string    `json:"chain"`
		IsMainnet   int64     `json:"is_mainnet"`
		RequestsNum int64     `json:"num_requests"`
		Day         time.Time `json:"day"`
	}

	AggregatedProviderDailyStat struct {
		Day               time.Time `json:"day"`
		Provider          string    `json:"provider"`
		Chain             string    `json:"chain"`
		IsMainnet         bool      `json:"is_mainnet"`
		TotalFreeRequests int64     `json:"total_free_requests"`
		TotalPaidRequests int64     `json:"total_paid_requests"`
	}

	AggregatedUsageDailyStat struct {
		Day                         time.Time `json:"day"`
		UsersTotal                  int64     `json:"users_total"`
		TotalFreeSubscriptions      int64     `json:"total_free_subscriptions"`
		TotalDeveloperSubscriptions int64     `json:"total_developer_subscriptions"`
		TotalAdvancedSubscriptions  int64     `json:"total_advanced_subscriptions"`
		TotalProSubscriptions       int64     `json:"total_pro_subscriptions"`
		TotalNotUsedMplx            int64     `json:"total_not_used_mplx"`
	}
)

func (s *Storage) DailyProviderPaidRequests() (result []ResponseProviderDailyRequests, err error) {
	// 1 is free plan(subscriptionID) from PostgreSQL
	sqlQuery := `SELECT
			provider,
			chain,
			is_mainnet,
			count(*) as num_requests,
			toDate(timestamp) as day
		FROM
			aura.stats
		WHERE
			day < toDate(now(),
			'Etc/UTC')
			AND toDate(timestamp) >= toDate(now() - INTERVAL 1 DAY,
			'Etc/UTC')
			and subscription_id != 1
		GROUP BY
			provider,
			chain,
			is_mainnet,
			day;`

	rows, err := s.conn.Query(sqlQuery)
	if err != nil {
		return result, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry ResponseProviderDailyRequests
		if err = rows.Scan(&entry.Provider, &entry.Chain, &entry.IsMainnet, &entry.RequestsNum, &entry.Day); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Storage) DailyProviderFreeRequests() (result []ResponseProviderDailyRequests, err error) {
	// 1 is free plan(subscriptionID) from PostgreSQL
	sqlQuery := `SELECT
			provider,
			chain,
			is_mainnet,
			count(*) as num_requests,
			toDate(timestamp) as day
		FROM
			aura.stats
		WHERE
			day < toDate(now(),
			'Etc/UTC')
			AND toDate(timestamp) >= toDate(now() - INTERVAL 1 DAY,
			'Etc/UTC')
			and subscription_id = 1
		GROUP BY
			provider,
			chain,
			is_mainnet,
			day;`

	rows, err := s.conn.Query(sqlQuery)
	if err != nil {
		return result, fmt.Errorf("query: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry ResponseProviderDailyRequests
		if err = rows.Scan(&entry.Provider, &entry.Chain, &entry.IsMainnet, &entry.RequestsNum, &entry.Day); err != nil {
			return nil, fmt.Errorf("scan: %s", err)
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Storage) InsertDailyProviderStat(providersStats []AggregatedProviderDailyStat) (err error) {
	if len(providersStats) == 0 {
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

	stmt, err := tx.Prepare(`INSERT INTO aggregated_providers_stats (
		time,
		provider,
		chain,
		is_mainnet,
		total_free_requests,
		total_paid_requests
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	for _, stat := range providersStats {
		_, err := stmt.Exec(
			stat.Day,
			stat.Provider,
			stat.Chain,
			stat.IsMainnet,
			stat.TotalFreeRequests,
			stat.TotalPaidRequests,
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

func (s *Storage) InsertAggregatedUsageDailyStat(usageStat AggregatedUsageDailyStat) (err error) {
	query := `INSERT INTO aggregated_usage_data 
		(
			time,
			users_total,
			total_free_subscriptions,
			total_developer_subscriptions,
			total_advanced_subscriptions,
			total_pro_subscriptions,
			total_not_used_mplx
		) VALUES 
		(
			?, ?, ?, ?, ?, ?, ?
		)`
	_, err = s.conn.Exec(query, usageStat.Day, usageStat.UsersTotal, usageStat.TotalFreeSubscriptions,
		usageStat.TotalDeveloperSubscriptions, usageStat.TotalAdvancedSubscriptions, usageStat.TotalProSubscriptions, usageStat.TotalNotUsedMplx)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}
