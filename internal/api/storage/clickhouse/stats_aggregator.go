package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/adm-metaex/aura-api/internal/pkg/log"
	"github.com/adm-metaex/aura-api/internal/pkg/util"
)

const aggregatorInterval = 12 * time.Hour

func (s *Storage) RunStatsAggregator(ctx context.Context) {
	// ignore err
	_ = util.AsyncRunWithInterval(ctx, nil, aggregatorInterval, true, false, func(_ context.Context) error {
		err := s.aggregateStatsData()
		if err != nil {
			log.Logger.Collector.Errorf("Storage.aggregateStatsData: %s", err)
		}
		return nil
	})
}

func (s *Storage) aggregateStatsData() error {
	err := s.AggregateUserData()
	if err != nil {
		return fmt.Errorf("AggregateUserData: %s", err)
	}
	err = s.AggregateAnalysisStats()
	if err != nil {
		return fmt.Errorf("AggregateAnalysisStats: %s", err)
	}
	err = s.DeleteOutdatedStats()
	if err != nil {
		return fmt.Errorf("DeleteOutdatedStats: %s", err)
	}

	return nil
}
