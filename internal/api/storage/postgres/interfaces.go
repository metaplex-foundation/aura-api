package postgres

import (
	"context"
	"time"
)

type UserSubscriptionStorage interface {
	GetCountOfSubscriptionUsersByDay(ctx context.Context, subscriptionId int, day time.Time) (int64, error)
	GetSubscrPriceAndDurationById(ctx context.Context, subscriptionId int) (SubscrPriceAndDuration, error)
	SaveProvidersRewards(ctx context.Context, rewards map[string]int64, day time.Time) error
	GetMaxAggregatedRewardsData(ctx context.Context) (*time.Time, error)
}
