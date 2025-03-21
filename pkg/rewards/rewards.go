package rewards

import (
	"context"
	"fmt"
	"time"

	"github.com/adm-metaex/aura-api/internal/models"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/util"
	"github.com/go-co-op/gocron"
	"github.com/shopspring/decimal"
)

const (
	// it has to be launched after stats.Collector
	usersCountStartTime = "03:00"

	advancedPlanId = 3
	proPlanId      = 4
)

type (
	DailyAggregatedRequestsWithWeights struct {
		Provider      string
		RequestType   string
		Chain         string
		WeightedUsage int64
		RequestPrice  int64
		RequestCount  int64
	}

	WeightedUsageRewPool struct {
		WeightedUsage int64
		RewardsPool   int64
	}

	AggregatedRequestInfo struct {
		RewardsPool     int64
		CountOfRequests int64
	}
)

type RewardsCalculator struct {
	pgStorage UserSubscriptionStorage
	chStorage UsageStatisticsStorage
}

type UserSubscriptionStorage interface {
	GetCountOfSubscriptionUsersByDay(ctx context.Context, subscriptionId int, day time.Time) (int64, error)
	GetSubscrPriceAndDurationById(ctx context.Context, subscriptionId int) (models.SubscrPriceAndDuration, error)
	SaveProvidersRewards(ctx context.Context, rewards map[string]int64, day time.Time) error
	GetMaxCalculatedRewardsData(ctx context.Context) (*time.Time, error)
}

type UsageStatisticsStorage interface {
	GetDailyPayAsYouGoRequests(ctx context.Context, day time.Time) ([]models.DailyAggregatedRequests, error)
	GetDailySubscriptionRequests(ctx context.Context, day time.Time) ([]models.DailyAggregatedRequests, error)
	GetProvidersRequestsServed(ctx context.Context, day time.Time) ([]models.DailyProvidersStat, error)
}

func New(pgStorage UserSubscriptionStorage, chStorage UsageStatisticsStorage) (c RewardsCalculator) {

	return RewardsCalculator{pgStorage: pgStorage, chStorage: chStorage}
}

func (c *RewardsCalculator) RunRewardsCalculation(ctx context.Context) {
	cron := gocron.NewScheduler(time.UTC)
	_, err := cron.Every(1).Day().At(usersCountStartTime).Do(func() {
		timeNow := time.Now().UTC()

		err := c.CalculateAndSaveRewards(ctx)
		if err != nil {
			log.Logger.Collector.Errorf("CalculateAndSaveRewards: %s", err)
		}

		log.Logger.Collector.Debugf("RunRewardsCalculation: time elapsed %s", time.Since(timeNow))
	})
	if err != nil {
		log.Logger.Collector.Fatalf("cron: %s", err)
	}

	cron.StartAsync()
}

func (c *RewardsCalculator) CalculateAndSaveRewards(ctx context.Context) (err error) {
	latestDayWithRewards, err := c.pgStorage.GetMaxCalculatedRewardsData(ctx)
	if err != nil {
		return fmt.Errorf("GetMaxCalculatedRewardsData: %w", err)
	}

	startDate := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	if latestDayWithRewards != nil {
		// rewards for yesterday already calculated
		if startDate == *latestDayWithRewards {
			return nil
		}

		// add 1 day because we should not recalculate rewards for dates we already processed
		startDate = latestDayWithRewards.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	}

	datesRange := util.GenerateDateRange(startDate)

	for _, date := range datesRange {
		providerRewards, err := c.CalculateRewardsForDay(ctx, date)
		if err != nil {
			return fmt.Errorf("CalculateRewardsForDay: %w", err)
		}

		err = c.pgStorage.SaveProvidersRewards(ctx, providerRewards, date)
		if err != nil {
			return fmt.Errorf("SaveProvidersRewards: %w", err)
		}
	}

	return nil
}

func (c *RewardsCalculator) CalculateRewardsForDay(ctx context.Context, day time.Time) (providerRewards map[string]int64, err error) {
	dailySubscrPlanRevenue, err := c.calculateDailySubscriptionRevenue(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("calculateDailySubscriptionRevenue: %w", err)
	}

	providerRewards = make(map[string]int64)

	generalRequestRewardsPool, err := c.processPayAsYouGoRewards(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("processPayAsYouGoRewards: %w", err)
	}

	generalRequestRewardsPool, err = c.processSubscriptionRewards(ctx, day, dailySubscrPlanRevenue, generalRequestRewardsPool)
	if err != nil {
		return nil, fmt.Errorf("processSubscriptionRewards: %w", err)
	}

	providerRewards, err = c.calculateProviderRewards(ctx, day, generalRequestRewardsPool)
	if err != nil {
		return nil, fmt.Errorf("calculateProviderRewards: %w", err)
	}

	return providerRewards, nil
}

func (c *RewardsCalculator) calculateDailySubscriptionRevenue(ctx context.Context, day time.Time) (int64, error) {
	countOfAdvancedPlanUsers, err := c.pgStorage.GetCountOfSubscriptionUsersByDay(ctx, advancedPlanId, day)
	if err != nil {
		return 0, fmt.Errorf("GetCountOfSubscriptionUsersByDay, advanced plan users: %w", err)
	}

	countOfProPlanUsers, err := c.pgStorage.GetCountOfSubscriptionUsersByDay(ctx, proPlanId, day)
	if err != nil {
		return 0, fmt.Errorf("GetCountOfSubscriptionUsersByDay pro plan users: %w", err)
	}

	advancedPlanPriceDuration, err := c.pgStorage.GetSubscrPriceAndDurationById(ctx, advancedPlanId)
	if err != nil {
		return 0, fmt.Errorf("GetSubscrPriceAndDurationById, advanced plan: %w", err)
	}

	var advancedPlanDailyRevenue int64
	if countOfAdvancedPlanUsers > 0 && advancedPlanPriceDuration.SbsDurationDays > 0 {
		advancedPlanDailyRevenue = countOfAdvancedPlanUsers * (advancedPlanPriceDuration.SbsPriceMplx / advancedPlanPriceDuration.SbsDurationDays)
	}

	proPlanPriceDuration, err := c.pgStorage.GetSubscrPriceAndDurationById(ctx, proPlanId)
	if err != nil {
		return 0, fmt.Errorf("GetSubscrPriceAndDurationById pro plan: %w", err)
	}

	var proPlanDailyRevenue int64
	if countOfProPlanUsers > 0 && proPlanPriceDuration.SbsDurationDays > 0 {
		proPlanDailyRevenue = countOfProPlanUsers * (proPlanPriceDuration.SbsPriceMplx / proPlanPriceDuration.SbsDurationDays)
	}

	return advancedPlanDailyRevenue + proPlanDailyRevenue, nil
}

func (c *RewardsCalculator) processPayAsYouGoRewards(ctx context.Context, day time.Time) (map[string]map[string]AggregatedRequestInfo, error) {
	aggregatedPayAsYouGoRequests, err := c.chStorage.GetDailyPayAsYouGoRequests(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("GetDailyPayAsYouGoRequests: %w", err)
	}

	generalRequestRewardsPool := make(map[string]map[string]AggregatedRequestInfo)
	payAsYouGoRewardsPool := make(map[string]map[string]int64)

	for _, req := range aggregatedPayAsYouGoRequests {
		ensureMapExists(payAsYouGoRewardsPool, req.Chain)
		ensureMapExists(generalRequestRewardsPool, req.Chain)

		requestRewards := req.RequestCount * req.RequestPrice
		payAsYouGoRewardsPool[req.Chain][req.RequestType] = requestRewards

		current := generalRequestRewardsPool[req.Chain][req.RequestType]
		generalRequestRewardsPool[req.Chain][req.RequestType] = AggregatedRequestInfo{
			RewardsPool:     current.RewardsPool + requestRewards,
			CountOfRequests: current.CountOfRequests + req.RequestCount,
		}
	}

	return generalRequestRewardsPool, nil
}

func (c *RewardsCalculator) processSubscriptionRewards(ctx context.Context, day time.Time, dailySubscrPlanRevenue int64, generalRequestRewardsPool map[string]map[string]AggregatedRequestInfo) (map[string]map[string]AggregatedRequestInfo, error) {
	aggregatedSubscriptionRequests, err := c.chStorage.GetDailySubscriptionRequests(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("GetDailySubscriptionRequests: %w", err)
	}

	subscriptionRewardsPool := make(map[string]map[string]WeightedUsageRewPool)

	var sumOfWeightedUsage int64

	for _, req := range aggregatedSubscriptionRequests {
		weightedUsage := decimal.NewFromInt(req.RequestCount).Mul(decimal.NewFromInt(req.RequestPrice)).Round(0).BigInt().Int64()
		sumOfWeightedUsage += weightedUsage

		ensureMapExists(subscriptionRewardsPool, req.Chain)
		subscriptionRewardsPool[req.Chain][req.RequestType] = WeightedUsageRewPool{WeightedUsage: weightedUsage}
	}

	for _, req := range aggregatedSubscriptionRequests {
		if chain, exists := subscriptionRewardsPool[req.Chain]; exists {
			if v, exists := chain[req.RequestType]; exists {
				if sumOfWeightedUsage == 0 {
					continue
				}

				ratio := decimal.NewFromInt(v.WeightedUsage).Div(decimal.NewFromInt(sumOfWeightedUsage))

				requestRewards := ratio.Mul(decimal.NewFromInt(dailySubscrPlanRevenue))
				v.RewardsPool = requestRewards.Round(0).BigInt().Int64()

				ensureMapExists(generalRequestRewardsPool, req.Chain)

				current := generalRequestRewardsPool[req.Chain][req.RequestType]
				generalRequestRewardsPool[req.Chain][req.RequestType] = AggregatedRequestInfo{
					RewardsPool:     current.RewardsPool + v.RewardsPool,
					CountOfRequests: current.CountOfRequests + req.RequestCount,
				}
			}
		}
	}

	return generalRequestRewardsPool, nil
}

func (c *RewardsCalculator) calculateProviderRewards(ctx context.Context, day time.Time, generalRequestRewardsPool map[string]map[string]AggregatedRequestInfo) (map[string]int64, error) {
	providersRequestsServed, err := c.chStorage.GetProvidersRequestsServed(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("GetProvidersRequestsServed: %w", err)
	}

	providerRewards := make(map[string]int64)

	for _, providerInfo := range providersRequestsServed {
		chainRewards, chainExists := generalRequestRewardsPool[providerInfo.Chain]
		if !chainExists {
			continue
		}

		requestTypeInfo, requestTypeExists := chainRewards[providerInfo.RequestType]
		if !requestTypeExists {
			continue
		}

		totalRequestRewardsPool := requestTypeInfo.RewardsPool
		totalRequestsWereMade := requestTypeInfo.CountOfRequests

		if totalRequestsWereMade == 0 {
			continue
		}

		providerServedRequests := providerInfo.RequestCount

		providerReqRewards := totalRequestRewardsPool * providerServedRequests / totalRequestsWereMade

		providerRewards[providerInfo.Provider] += providerReqRewards
	}

	return providerRewards, nil
}

func ensureMapExists[K comparable, V any](m map[K]map[string]V, key K) {
	if _, exists := m[key]; !exists {
		m[key] = make(map[string]V)
	}
}
