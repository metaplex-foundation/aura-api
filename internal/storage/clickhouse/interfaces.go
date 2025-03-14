package clickhouse

import (
	"context"
	"time"
)

type UsageStatisticsStorage interface {
	GetDailyPayAsYouGoRequests(ctx context.Context, day time.Time) ([]DailyAggregatedRequests, error)
	GetDailySubscriptionRequests(ctx context.Context, day time.Time) ([]DailyAggregatedRequests, error)
	GetProvidersRequestsServed(ctx context.Context, day time.Time) ([]DailyProvidersStat, error)
}
