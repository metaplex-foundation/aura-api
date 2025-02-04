package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/go-co-op/gocron"
)

const (
	dailyCollectorStartTime = "00:10"
)

type Collector struct {
	pgStorage postgres.Storage
	chStorage clickhouse.Storage
}

func New(pgStorage postgres.Storage, chStorage clickhouse.Storage) (c Collector) {
	return Collector{pgStorage: pgStorage, chStorage: chStorage}
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
	err = c.collectAndSaveUserSubscrStat(ctx)
	if err != nil {
		return fmt.Errorf("collectAndSaveUsageStat: %s", err)
	}

	err = c.collectAndSaveProvidersStat()
	if err != nil {
		return fmt.Errorf("collectAndSaveProvidersStat: %s", err)
	}

	return nil
}

func (c *Collector) collectAndSaveUserSubscrStat(ctx context.Context) (err error) {
	usersTotal, err := c.pgStorage.GetUsersCount(ctx)
	if err != nil {
		return fmt.Errorf("GetUsersCount: %s", err)
	}

	usersSubscriptionsCount, err := c.pgStorage.GetUsersSubscriptionsCount(ctx)
	if err != nil {
		return fmt.Errorf("GetUsersSubscriptionsCount: %s", err)
	}

	usersAvailableMPLXBalance, err := c.pgStorage.GetUsersAvailableMPLXBalance(ctx)
	if err != nil {
		return fmt.Errorf("GetUsersAvailableMPLXBalance: %s", err)
	}

	usageDailyStat := clickhouse.AggregatedUsageDailyStat{}

	for _, reqCount := range usersSubscriptionsCount {
		switch reqCount.SubscriptionId {
		case 1:
			usageDailyStat.TotalFreeSubscriptions += reqCount.SubscriptionCount
		case 2:
			usageDailyStat.TotalDeveloperSubscriptions += reqCount.SubscriptionCount
		case 3:
			usageDailyStat.TotalAdvancedSubscriptions += reqCount.SubscriptionCount
		case 4:
			usageDailyStat.TotalProSubscriptions += reqCount.SubscriptionCount
		default:
			return fmt.Errorf("got unknown users subscription ID")
		}
	}

	usageDailyStat.Day = time.Now()
	usageDailyStat.UsersTotal = usersTotal
	usageDailyStat.TotalNotUsedMplx = usersAvailableMPLXBalance

	err = c.chStorage.InsertAggregatedUsageDailyStat(usageDailyStat)
	if err != nil {
		return fmt.Errorf("InsertAggregatedUsageDailyStat: %s", err)
	}
	return nil
}

func (c *Collector) collectAndSaveProvidersStat() (err error) {
	numOfPaidRequests, err := c.chStorage.DailyProviderPaidRequests()
	if err != nil {
		return fmt.Errorf("DailyProviderPaidRequests: %s", err)
	}

	numOfFreeRequests, err := c.chStorage.DailyProviderFreeRequests()
	if err != nil {
		return fmt.Errorf("DailyProviderFreeRequests: %s", err)
	}

	// provider chain isMainnet AggregatedProviderDailyStat
	// don't put day to the map because all the queries select data for one day
	resultMap := make(map[string]map[string]map[bool]clickhouse.AggregatedProviderDailyStat)

	updateResultMap(resultMap, numOfPaidRequests, func(stat *clickhouse.AggregatedProviderDailyStat, e clickhouse.ResponseProviderDailyRequests) {
		stat.TotalPaidRequests += e.RequestsNum
	})
	updateResultMap(resultMap, numOfFreeRequests, func(stat *clickhouse.AggregatedProviderDailyStat, e clickhouse.ResponseProviderDailyRequests) {
		stat.TotalFreeRequests += e.RequestsNum
	})

	var statsSlice []clickhouse.AggregatedProviderDailyStat
	for _, chainMap := range resultMap {
		for _, isMainnetMap := range chainMap {
			for _, stat := range isMainnetMap {
				statsSlice = append(statsSlice, stat)
			}
		}
	}

	err = c.chStorage.InsertDailyProviderStat(statsSlice)
	if err != nil {
		return fmt.Errorf("InsertDailyProviderStat: %s", err)
	}

	return nil
}

func updateResultMap(
	resultMap map[string]map[string]map[bool]clickhouse.AggregatedProviderDailyStat,
	elements []clickhouse.ResponseProviderDailyRequests,
	updateFunc func(*clickhouse.AggregatedProviderDailyStat, clickhouse.ResponseProviderDailyRequests)) {
	for _, element := range elements {
		provider, chain, isMainnet := element.Provider, element.Chain, element.IsMainnet == 1

		if _, ok := resultMap[provider]; !ok {
			resultMap[provider] = make(map[string]map[bool]clickhouse.AggregatedProviderDailyStat)
		}
		if _, ok := resultMap[provider][chain]; !ok {
			resultMap[provider][chain] = make(map[bool]clickhouse.AggregatedProviderDailyStat)
		}

		stat, exists := resultMap[provider][chain][isMainnet]
		if !exists {
			stat = clickhouse.AggregatedProviderDailyStat{
				Day:       element.Day,
				Provider:  provider,
				Chain:     chain,
				IsMainnet: isMainnet,
			}
		}

		updateFunc(&stat, element)

		resultMap[provider][chain][isMainnet] = stat
	}
}
