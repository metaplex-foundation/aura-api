package postgres

import (
	"context"
	"fmt"
	"time"
)

type (
	Plan struct {
		Name              string    `pg:"sbs_name" json:"name"`
		RequestsPerSecond int64     `pg:"sbs_request_per_second" json:"requests_per_second"`
		TokenLimit        int64     `pg:"sbs_tokens_limit" json:"token_limit"`
		Priority          int64     `pg:"sbs_priority" json:"priority"`
		CreatedAt         time.Time `pg:"sbs_created_at" json:"-"`
	}
)

const (
	subscriptionsTable = "subscriptions"
)

func (s *Storage) GetSubscriptionsList(ctx context.Context) (subscriptions []Plan, err error) {
	query := `SELECT sbs_priority, sbs_name, sbs_request_per_second, sbs_tokens_limit, sbs_created_at
				FROM subscriptions`
	_, err = s.db.QueryContext(ctx, &subscriptions, query)
	if err != nil {
		return subscriptions, fmt.Errorf("QueryContext: %w", err)
	}

	return subscriptions, nil
}
