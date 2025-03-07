package postgres

import (
	"context"
	"fmt"
	"time"
)

type (
	Plan struct {
		PlanID     int64     `pg:"sbs_id" json:"id"`
		Name       string    `pg:"sbs_name" json:"name"`
		TokenLimit int64     `pg:"sbs_tokens_limit" json:"token_limit"`
		Priority   int64     `pg:"sbs_priority" json:"priority"`
		CreatedAt  time.Time `pg:"sbs_created_at" json:"-"`
	}

	SubscrPriceAndDuration struct {
		SbsPriceMplx    int64 `pg:"sbs_price_mplx" json:"sbs_price_mplx"`
		SbsDurationDays int64 `pg:"sbs_period_days" json:"sbs_period_days"`
	}
)

const (
	subscriptionsTable = "subscriptions"
)

func (s *Storage) GetSubscriptionsList(ctx context.Context) (subscriptions []Plan, err error) {
	query := `SELECT sbs_id, sbs_priority, sbs_name, sbs_tokens_limit, sbs_created_at
				FROM subscriptions`
	_, err = s.db.QueryContext(ctx, &subscriptions, query)
	if err != nil {
		return subscriptions, fmt.Errorf("QueryContext: %w", err)
	}

	return subscriptions, nil
}

func (s *Storage) GetSubscrPriceAndDurationById(ctx context.Context, subscriptionId int) (sbsInfo SubscrPriceAndDuration, err error) {
	query := `SELECT sbs_price_mplx, sbs_period_days FROM subscriptions WHERE sbs_id = ?;`
	_, err = s.db.QueryContext(ctx, &sbsInfo, query, subscriptionId)
	if err != nil {
		return sbsInfo, fmt.Errorf("QueryContext: %w", err)
	}

	return sbsInfo, nil
}
