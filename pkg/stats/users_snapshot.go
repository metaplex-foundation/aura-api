package stats

import (
	"context"
	"time"

	"github.com/adm-metaex/aura-api/internal/models"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/metrics"
	"github.com/go-co-op/gocron"
)

const (
	dailyUserSnapshotStartTime = "00:30" // has to work after clickhouse stats aggregator
)

type UsersSnapshot struct {
	pgStorage UserStorage
	chStorage ActiveUsersSource
}

type ActiveUsersSource interface {
	GetNumberOfActiveUsersForDay(context.Context, time.Time) (int64, error)
}
type UserStorage interface {
	GetCurrentUsersCountByPlans(context.Context) ([]models.UserCountByPlan, error)
	SaveUsersSnapshot(context.Context, models.UsersSnapshot) error
	EnhanceUsersSnapshotWithActiveUsers(context.Context, time.Time, int64) error
}

func NewUsersSnapshotJob(pgStorage UserStorage, chStorage ActiveUsersSource) (c UsersSnapshot) {

	return UsersSnapshot{pgStorage: pgStorage, chStorage: chStorage}
}

func (c *UsersSnapshot) RunUsersSnapshotJob(ctx context.Context) {
	cron := gocron.NewScheduler(time.UTC)
	_, err := cron.Every(1).Day().At(dailyUserSnapshotStartTime).Do(func() {
		timeNow := time.Now()

		err := c.enhanceSnapshot(ctx)
		if err != nil {
			log.Logger.Collector.Errorf("enhanceSnapshot: %s", err)
		}

		duration := time.Since(timeNow)

		metrics.ObserveBackgroundWorkerExecutionTime("UsersSnapshotJob", duration)
		log.Logger.Collector.Debugf("enhanceSnapshot: time elapsed %s", duration)
	})
	if err != nil {
		log.Logger.Collector.Fatalf("cron: %s", err)
	}

	cron.StartAsync()
}

func (c *UsersSnapshot) enhanceSnapshot(ctx context.Context) error {
	yesterday := time.Now().AddDate(0, 0, -1).UTC().Truncate(24 * time.Hour)

	numberOfActiveUsers, err := c.chStorage.GetNumberOfActiveUsersForDay(ctx, yesterday)
	if err != nil {
		return err
	}

	err = c.pgStorage.EnhanceUsersSnapshotWithActiveUsers(ctx, yesterday, numberOfActiveUsers)
	if err != nil {
		return err
	}

	return err
}

func InitializeUsersSnapshot(ctx context.Context, storage UserStorage) error {
	usersByPlans, err := storage.GetCurrentUsersCountByPlans(ctx)
	if err != nil {
		return err
	}

	yesterday := time.Now().AddDate(0, 0, -1).UTC()

	snapshot := models.UsersSnapshot{
		Day: yesterday.Truncate(24 * time.Hour),
	}

	for _, data := range usersByPlans {
		switch data.SubscriptionId {
		case 4: // Pro
			{
				snapshot.ProSubscriptions += data.Count
			}
		case 3: // Advanced
			{
				snapshot.AdvancedSubscriptions += data.Count
			}
		case 2: // Developer or pay-as-you-go
			{
				snapshot.PayAsYouGo += data.Count
			}
		default:
			{
			}
		}

		snapshot.TotalUsers += data.Count
	}

	err = storage.SaveUsersSnapshot(ctx, snapshot)

	return err
}
