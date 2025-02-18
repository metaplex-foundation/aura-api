package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/configtypes"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/go-co-op/gocron"
	consulAPI "github.com/hashicorp/consul/api"
	"github.com/shopspring/decimal"
)

const (
	dailyCollectorStartTime = "00:10"
	metaplexTokenDecimals   = 6
)

var (
	metaplexTokenDecimalsMultiplier = decimal.NewFromFloat(10).Pow(decimal.NewFromFloat(metaplexTokenDecimals))
)

type Collector struct {
	pgStorage    postgres.Storage
	chStorage    clickhouse.Storage
	consulClient consulAPI.Client
}

func New(pgStorage postgres.Storage, chStorage clickhouse.Storage, consulClient consulAPI.Client) (c Collector) {

	return Collector{pgStorage: pgStorage, chStorage: chStorage, consulClient: consulClient}
}

func generateDateRange(maxDay time.Time) []time.Time {
	var dates []time.Time
	today := time.Now().Truncate(24 * time.Hour)
	maxDay = maxDay.Truncate(24 * time.Hour)

	// start from the day after maxDay
	for d := maxDay.AddDate(0, 0, 1); d.Before(today); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}

	return dates
}

func (c *Collector) RunStatsCollector(ctx context.Context) {
	cron := gocron.NewScheduler(time.UTC)
	_, err := cron.Every(1).Day().At(dailyCollectorStartTime).Do(func() {
		timeNow := time.Now()

		err := c.collectLatestStat(ctx)
		if err != nil {
			log.Logger.Collector.Errorf("collectLatestStat: %s", err)
		}

		log.Logger.Collector.Debugf("collectLatestStat: time elapsed %s", time.Since(timeNow))
	})
	if err != nil {
		log.Logger.Collector.Fatalf("cron: %s", err)
	}

	cron.StartAsync()
}

func (c *Collector) collectLatestStat(ctx context.Context) (err error) {
	err = c.aggregateDataForPayAsYouGoPlan(ctx)
	if err != nil {
		return fmt.Errorf("aggregateDataForPayAsYouGoPlan: %s", err)
	}

	err = c.aggregateDataForSubscriptionPlan(ctx)
	if err != nil {
		return fmt.Errorf("aggregateDataForSubscriptionPlan: %s", err)
	}

	return nil
}

func (c *Collector) aggregateDataForPayAsYouGoPlan(ctx context.Context) error {
	latestAggregatedDay, err := c.chStorage.GetLatestAggregatedRewardsDayByPlan(ctx, clickhouse.PayAsYouGo)
	if err != nil {
		return fmt.Errorf("GetLatestAggregatedRewardsDayByPlan: %s", err)
	}

	// determine start date for aggregation
	startDate := time.Now().AddDate(0, 0, -(clickhouse.OutdatedStatsPeriod + 1))
	if latestAggregatedDay != nil {
		startDate = latestAggregatedDay.Time
	}

	// generate date range
	datesRange := generateDateRange(startDate)

	var allStats []clickhouse.ProviderRequestStats

	for _, date := range datesRange {
		aggregatedStats, err := c.chStorage.GetProviderRequestStatsPayAsYouGoPlan(ctx, clickhouse.Date{date})
		if err != nil {
			return fmt.Errorf("GetProviderRequestStatsPayAsYouGoPlan for date %s: %s", date.Format("2006-01-02"), err)
		}
		allStats = append(allStats, aggregatedStats...)
	}

	if len(allStats) == 0 {
		return nil
	}

	if err := c.chStorage.SaveAggregatedProvidersStat(ctx, allStats, clickhouse.PayAsYouGo); err != nil {
		return fmt.Errorf("SaveAggregatedProvidersStat: %s", err)
	}

	return nil
}

func (c *Collector) aggregateDataForSubscriptionPlan(ctx context.Context) (err error) {
	latestAggregatedDay, err := c.chStorage.GetLatestAggregatedRewardsDayByPlan(ctx, clickhouse.Subscription)
	if err != nil {
		return fmt.Errorf("GetLatestAggregatedRewardsDayByPlan: %s", err)
	}

	// determine start date for aggregation
	startDate := time.Now().AddDate(0, 0, -(clickhouse.OutdatedStatsPeriod + 1))
	if latestAggregatedDay != nil {
		startDate = latestAggregatedDay.Time
	}

	// generate date range
	datesRange := generateDateRange(startDate)

	var allStats []clickhouse.ProviderRequestStats

	for _, date := range datesRange {
		aggregatedSubscriptionStat, err := c.chStorage.GetProviderRequestStatsSubscriptionPlan(ctx, clickhouse.Date{date})
		if err != nil {
			return fmt.Errorf("GetProviderRequestStatsSubscriptionPlan: %s", err)
		}

		allStats = append(allStats, aggregatedSubscriptionStat...)
	}

	// for subscription plan queries we take it's prices from Consul
	// so even if price was changing during the day we will take only price at the end of a day
	consulKV := c.consulClient.KV()
	pair, _, err := consulKV.Get(configtypes.ConsulPricingPath, nil)
	if err != nil {
		return fmt.Errorf("consulKV.Get %s: %s", configtypes.ConsulPricingPath, err)
	}
	if pair == nil || pair.Value == nil {
		return fmt.Errorf("consulKV.Get %s: returned nil value", configtypes.ConsulPricingPath)
	}

	var pricing configtypes.PricingPlans
	if err = json.Unmarshal(pair.Value, &pricing); err != nil {
		return fmt.Errorf("PricingConfig: json.Unmarshal: %s", err)
	}

	pair, _, err = consulKV.Get(configtypes.ConsulMplxPricePath, nil)
	if err != nil {
		return fmt.Errorf("consulKV.Get %s: %s", configtypes.ConsulMplxPricePath, err)
	}
	if pair == nil || pair.Value == nil {
		return fmt.Errorf("consulKV.Get %s: returned nil value", configtypes.ConsulMplxPricePath)
	}
	mplxUSDPrice, err := decimal.NewFromString(strings.TrimSpace(string(pair.Value)))
	if err != nil {
		return fmt.Errorf("NewFromString: %s", err)
	}

	chainRequestTypePrice := make(map[string]map[string]int64)

	// helper function to assign values
	assignPrice := func(chain, requestType string, pricingModel configtypes.PricingModel) {
		if _, exists := chainRequestTypePrice[chain]; !exists {
			chainRequestTypePrice[chain] = make(map[string]int64)
		}
		// convert request prices from USD to MPLX
		mplxRequestPrice := pricingModel.PriceUSD.Div(mplxUSDPrice).Truncate(metaplexTokenDecimals).Mul(metaplexTokenDecimalsMultiplier).Floor().BigInt().Int64()
		chainRequestTypePrice[chain][requestType] = mplxRequestPrice
	}

	// extract prices
	// taking Pro subscription prices even though they are same as Advanced
	assignPrice("solana", "DAS", pricing.Pro.SolanaDAS)
	assignPrice("eclipse", "DAS", pricing.Pro.EclipseDAS)
	assignPrice("solana", "RPC", pricing.Pro.SolanaRPC)
	assignPrice("eclipse", "RPC", pricing.Pro.EclipseRPC)
	assignPrice("solana", "GPA", pricing.Pro.SolanaGetProgramAccounts)
	assignPrice("eclipse", "GPA", pricing.Pro.EclipseGetProgramAccounts)
	assignPrice("solana", "SWQOS", pricing.Pro.SolanaSWQOS)
	assignPrice("eclipse", "SWQOS", pricing.Pro.EclipseSWQOS)
	assignPrice("solana", "Websocket", pricing.Pro.SolanaWebsocket)
	assignPrice("eclipse", "Websocket", pricing.Pro.EclipseWebsocket)

	// set price for each selected aggregated stat
	for i := range allStats {
		chain := allStats[i].Chain
		requestType := allStats[i].RequestType

		price, exists := chainRequestTypePrice[chain][requestType]
		if !exists {
			return fmt.Errorf("missing price at Consul for chain: %s, request type: %s", chain, requestType)
		}

		allStats[i].RequestPrice = price
	}

	err = c.chStorage.SaveAggregatedProvidersStat(ctx, allStats, clickhouse.Subscription)
	if err != nil {
		return fmt.Errorf("SaveAggregatedProvidersStat: %s", err)
	}

	return nil
}
