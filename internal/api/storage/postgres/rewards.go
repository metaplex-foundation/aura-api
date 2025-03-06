package postgres

import (
	"context"
	"fmt"
	"time"
)

type (
	Date struct {
		Day *time.Time
	}

	DailyRewardsPaid struct {
		Provider string    `pg:"provider"`
		Volume   int64     `pg:"volume"`
		Day      time.Time `pg:"day"`
	}
)

func (s *Storage) SaveProvidersRewards(ctx context.Context, rewards map[string]int64, day time.Time) (err error) {
	if len(rewards) == 0 {
		return nil
	}

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginTx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	query := `INSERT INTO providers_rewards (prw_provider, prw_rewards, prw_day) VALUES (?, ?, ?);`

	for provider, rewards := range rewards {
		_, err = tx.db.ExecOne(query, provider, rewards, day)
		if err != nil {
			return fmt.Errorf("ExecOne: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (s *Storage) GetMaxCalculatedRewardsData(ctx context.Context) (day *time.Time, err error) {
	query := `SELECT MAX(prw_day) AS day FROM providers_rewards;`

	var result Date
	_, err = s.db.QueryOneContext(ctx, &result, query)
	if err != nil {
		return day, err
	}

	return result.Day, nil
}

func (s *Storage) GetTotalMPLXDistributed(ctx context.Context) (volume int64, err error) {
	query := `SELECT SUM(pr.prw_rewards) AS volume FROM rewards_transactions rt
				JOIN providers_rewards pr ON rt.rwd_provider = pr.prw_provider 
				AND rt.rwd_rewards_day = pr.prw_day;`
	var result Volume
	_, err = s.db.QueryOneContext(ctx, &result, query)
	if err != nil {
		return result.Volume, err
	}

	return result.Volume, nil
}

func (s *Storage) GetDailyMPLXDistributed(ctx context.Context, startDay time.Time, endDay time.Time) (result []DailyRewardsPaid, err error) {
	if startDay.After(endDay) {
		return result, fmt.Errorf("failed to get daily MPLX rewards distribution because start day cannot be gibber than end data: %w and %w", startDay, endDay)
	}

	query := `SELECT 
			rt.rwd_provider as provider,
			SUM(pr.prw_rewards) AS volume,
			rt.rwd_paid_at as day
		FROM 
			rewards_transactions rt
		JOIN 
			providers_rewards pr ON rt.rwd_provider = pr.prw_provider 
			AND rt.rwd_rewards_day = pr.prw_day
		WHERE 
			rt.rwd_paid_at BETWEEN ? AND ?
		GROUP BY 
			rt.rwd_paid_at,
			rt.rwd_provider
		ORDER BY 
			rt.rwd_paid_at;`

	_, err = s.db.QueryContext(ctx, &result, query, startDay.Truncate(24*time.Hour), endDay.Truncate(24*time.Hour))
	if err != nil {
		return result, err
	}

	return result, nil
}
