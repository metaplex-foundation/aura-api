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
			return fmt.Errorf("ExecOne: %s", err)
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
