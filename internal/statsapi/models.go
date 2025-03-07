package statsapi

import (
	"net/http"
	"time"

	"github.com/adm-metaex/aura-api/internal/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/storage/postgres"
	"github.com/labstack/echo/v4"
)

type (
	TotalPaidMPLXResponse struct {
		PaidTotal int64 `json:"paid_total"`
	}

	PaidMPLXDailyResponse struct {
		PaidDaily []postgres.DailyVolume `json:"paid_daily"`
	}

	TotalDistributedMPLXResponse struct {
		DistributedTotal int64 `json:"distributed_total"`
	}

	DistributedMPLXDailyResponse struct {
		DistributedDaily []postgres.DailyRewardsPaid `json:"distributed_daily"`
	}

	TotalRewardsEarnedResponse struct {
		TotalEarned int64 `json:"total_earned"`
	}

	DailyRewardsEarnedResponse struct {
		DailyRewardsEarned []postgres.DailyEarnedRewards `json:"daily_rewards_earned"`
	}

	DailyRequestsResponse struct {
		DailyRequests []clickhouse.RequestsByChainAndType `json:"daily_requests"`
	}

	DailyUsersSnapshotResponse struct {
		DailyUsersSnapshot []postgres.UsersSnapshot `json:"daily_users_snapshot"`
	}
)

type (
	StartAndEndDatesParams struct {
		StartDay time.Time `json:"start_day"`
		EndDay   time.Time `json:"end_day"`
	}
)

func (p *StartAndEndDatesParams) Validate() error {
	if p.StartDay.After(p.EndDay) {
		return echo.NewHTTPError(http.StatusBadRequest, "Start day is greater then end day")
	}

	return nil
}
