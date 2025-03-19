package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron"

	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/metrics"
	"github.com/adm-metaex/aura-api/pkg/util"
)

const (
	hourlyAggregatorInterval = time.Hour
	dailyAggregatorStartTime = "00:05"
)

func (s *Storage) RunStatsAggregator(ctx context.Context) {
	cron := gocron.NewScheduler(time.UTC)
	_, err := cron.Every(1).Day().At(dailyAggregatorStartTime).Do(func() {
		timeNow := time.Now()
		err := s.aggregateStatsData(ctx)
		if err != nil {
			log.Logger.Collector.Errorf("aggregateStatsData: %s", err)
		}

		duration := time.Since(timeNow)

		metrics.ObserveBackgroundWorkerExecutionTime("StatsAggregator", duration)
		log.Logger.Collector.Debugf("aggregateStatsData: time elapsed %s", duration)
	})
	if err != nil {
		log.Logger.Collector.Fatalf("cron: %s", err)
	}

	cron.StartAsync()
	_ = util.AsyncRunWithInterval(ctx, nil, hourlyAggregatorInterval, false, false, func(ctx context.Context) error {
		err := s.AggregateUserDataHourly(ctx, true)
		if err != nil {
			log.Logger.Collector.Errorf("AggregateUserDataHourly: %s", err)
		}
		return nil
	})
}

func (s *Storage) RunInitialAggregation(ctx context.Context) error {
	err := s.AggregateUserDataDaily(ctx, false)
	if err != nil {
		return fmt.Errorf("AggregateUserDataDaily: %s", err)
	}
	err = s.AggregateUserDataHourly(ctx, false)
	if err != nil {
		return fmt.Errorf("AggregateUserDataHourly: %s", err)
	}
	err = s.AggregateAnalysisData(ctx, false)
	if err != nil {
		return fmt.Errorf("AggregateAnalysisData: %s", err)
	}
	err = s.DeleteOutdatedStats(ctx)
	if err != nil {
		return fmt.Errorf("DeleteOutdatedStats: %s", err)
	}
	err = s.DeleteOutdatedHourlyData(ctx)
	if err != nil {
		return fmt.Errorf("DeleteOutdatedHourlyData: %s", err)
	}

	return nil
}

func (s *Storage) aggregateStatsData(ctx context.Context) error {
	err := s.AggregateUserDataDaily(ctx, true)
	if err != nil {
		return fmt.Errorf("AggregateUserDataDaily: %s", err)
	}
	err = s.AggregateAnalysisData(ctx, true)
	if err != nil {
		return fmt.Errorf("AggregateAnalysisData: %s", err)
	}
	err = s.DeleteOutdatedStats(ctx)
	if err != nil {
		return fmt.Errorf("DeleteOutdatedStats: %s", err)
	}
	err = s.DeleteOutdatedHourlyData(ctx)
	if err != nil {
		return fmt.Errorf("DeleteOutdatedHourlyData: %s", err)
	}

	return nil
}
