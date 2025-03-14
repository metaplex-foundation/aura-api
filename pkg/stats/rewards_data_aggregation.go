package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adm-metaex/aura-api/internal/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/configtypes"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/util"
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

func (c *Collector) RunStatsCollector(ctx context.Context) {
	cron := gocron.NewScheduler(time.UTC)
	_, err := cron.Every(1).Day().At(dailyCollectorStartTime).Do(func() {
		timeNow := time.Now()

		err := c.CollectLatestStat(ctx)
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

func (c *Collector) CollectLatestStat(ctx context.Context) (err error) {
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
	startDate := time.Now().UTC().AddDate(0, 0, -(clickhouse.OutdatedStatsPeriod + 1)).Truncate(24 * time.Hour)
	if latestAggregatedDay != nil {
		// we should not re-aggregate data for yesterday
		// and we should exit the func because script can aggregate part of the today's data, which we should avoid
		yesterday := time.Now().UTC().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		if latestAggregatedDay.Time.Truncate(24*time.Hour) == yesterday {
			return nil
		}

		// add 1 day because we should not re-aggregate data for dates we already did it
		startDate = latestAggregatedDay.Time.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	}

	// generate date range
	datesRange := util.GenerateDateRange(startDate)

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
	startDate := time.Now().UTC().AddDate(0, 0, -(clickhouse.OutdatedStatsPeriod + 1)).Truncate(24 * time.Hour)
	if latestAggregatedDay != nil {
		// we should not re-aggregate data for yesterday
		// and we should exit the func because script can aggregate part of the today's data, which we should avoid
		yesterday := time.Now().UTC().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		if latestAggregatedDay.Time.Truncate(24*time.Hour) == yesterday {
			return nil
		}

		// add 1 day because we should not re-aggregate data for dates we already did it
		startDate = latestAggregatedDay.Time.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	}

	// generate date range
	datesRange := util.GenerateDateRange(startDate)

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
	pair, _, err := consulKV.Get(configtypes.ConsulMethodTypesPricesPath, nil)
	if err != nil {
		return fmt.Errorf("consulKV.Get %s: %s", configtypes.ConsulMethodTypesPricesPath, err)
	}
	if pair == nil || pair.Value == nil {
		return fmt.Errorf("consulKV.Get %s: returned nil value", configtypes.ConsulMethodTypesPricesPath)
	}

	var pricing configtypes.PricingConfig
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
	assignPrice("solana", "DAS", pricing.SolanaDAS)
	assignPrice("eclipse", "DAS", pricing.EclipseDAS)
	assignPrice("solana", "RPC", pricing.SolanaRPC)
	assignPrice("eclipse", "RPC", pricing.EclipseRPC)
	assignPrice("solana", "GPA", pricing.SolanaGetProgramAccounts)
	assignPrice("eclipse", "GPA", pricing.EclipseGetProgramAccounts)
	assignPrice("solana", "SWQOS", pricing.SolanaSWQOS)
	assignPrice("eclipse", "SWQOS", pricing.EclipseSWQOS)
	assignPrice("solana", "Websocket", pricing.SolanaWebsocket)
	assignPrice("eclipse", "Websocket", pricing.EclipseWebsocket)

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
