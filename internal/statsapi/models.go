package statsapi

import (
	"net/http"
	"strings"
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
		PaidDaily []struct {
			Volume int64     `json:"volume"`
			Day    time.Time `json:"day"`
		} `json:"paid_daily"`
	}

	TotalDistributedMPLXResponse struct {
		DistributedTotal int64 `json:"distributed_total"`
	}

	DistributedMPLXDailyResponse struct {
		DistributedDaily []struct {
			Provider string    `json:"provider"`
			Paid     int64     `json:"paid"`
			Day      time.Time `json:"day"`
		} `json:"distributed_daily"`
	}

	TotalRewardsEarnedResponse struct {
		TotalEarned int64 `json:"total_earned"`
	}

	DailyRewardsEarnedResponse struct {
		DailyRewardsEarned []struct {
			Earned int64     `json:"earned"`
			Day    time.Time `json:"day"`
		} `json:"daily_rewards_earned"`
	}

	DailyRequestsResponse struct {
		DailyRequests []struct {
			Day         time.Time `json:"day"`
			Chain       string    `json:"chain"`
			RequestType string    `json:"request_type"`
			Requests    int64     `json:"requests"`
		} `json:"daily_requests"`
	}

	DailyUsersSnapshotResponse struct {
		DailyUsersSnapshot []struct {
			Day                   time.Time `json:"day"`
			ProSubscriptions      int64     `json:"pro_subscriptions"`
			AdvancedSubscriptions int64     `json:"advanced_subscriptions"`
			PayAsYouGo            int64     `json:"pay_as_you_go"`
			ActiveUsers           int64     `json:"active_users"`
			TotalUsers            int64     `json:"total_users"`
		} `json:"daily_users_snapshot"`
	}
)

type (
	StartAndEndDatesParams struct {
		StartDay Date `json:"start_day"`
		EndDay   Date `json:"end_day"`
	}
)

type Date struct {
	time.Time
}

func (ct *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}

func (p *StartAndEndDatesParams) Validate() error {
	if p.StartDay.After(p.EndDay.Time) {
		return echo.NewHTTPError(http.StatusBadRequest, "Start day is greater then end day")
	}

	return nil
}

func getPaidMPLXDailyResponse(initialData []postgres.DailyVolume) (result PaidMPLXDailyResponse) {
	for _, el := range initialData {
		result.PaidDaily = append(result.PaidDaily, struct {
			Volume int64     `json:"volume"`
			Day    time.Time `json:"day"`
		}{Volume: el.Volume, Day: el.Day})
	}
	return result
}

func getDistributedMPLXDailyResponse(initialData []postgres.DailyRewardsPaid) (result DistributedMPLXDailyResponse) {
	for _, el := range initialData {
		result.DistributedDaily = append(result.DistributedDaily, struct {
			Provider string    "json:\"provider\""
			Paid     int64     "json:\"paid\""
			Day      time.Time "json:\"day\""
		}{Provider: el.Provider, Paid: el.Paid, Day: el.Day})
	}

	return result
}

func getDailyRewardsEarnedResponse(initialData []postgres.DailyEarnedRewards) (result DailyRewardsEarnedResponse) {
	for _, el := range initialData {
		result.DailyRewardsEarned = append(result.DailyRewardsEarned, struct {
			Earned int64     "json:\"earned\""
			Day    time.Time "json:\"day\""
		}{Earned: el.Earned, Day: el.Day})
	}

	return result
}

func getDailyRequestsResponse(initialData []clickhouse.RequestsByChainAndType) (result DailyRequestsResponse) {
	for _, el := range initialData {
		result.DailyRequests = append(result.DailyRequests, struct {
			Day         time.Time "json:\"day\""
			Chain       string    "json:\"chain\""
			RequestType string    "json:\"request_type\""
			Requests    int64     "json:\"requests\""
		}{Day: el.Day, Chain: el.Chain, RequestType: el.RequestType, Requests: el.Requests})
	}

	return result
}

func getDailyUsersSnapshotResponse(initialData []postgres.UsersSnapshot) (result DailyUsersSnapshotResponse) {
	for _, el := range initialData {
		result.DailyUsersSnapshot = append(result.DailyUsersSnapshot, struct {
			Day                   time.Time "json:\"day\""
			ProSubscriptions      int64     "json:\"pro_subscriptions\""
			AdvancedSubscriptions int64     "json:\"advanced_subscriptions\""
			PayAsYouGo            int64     "json:\"pay_as_you_go\""
			ActiveUsers           int64     "json:\"active_users\""
			TotalUsers            int64     "json:\"total_users\""
		}{Day: el.Day, ProSubscriptions: el.ProSubscriptions, AdvancedSubscriptions: el.AdvancedSubscriptions,
			PayAsYouGo: el.PayAsYouGo, ActiveUsers: el.ActiveUsers, TotalUsers: el.TotalUsers})
	}

	return result
}
