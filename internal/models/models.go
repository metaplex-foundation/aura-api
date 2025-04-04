package models

import "time"

// Postgre models
type (
	SubscrPriceAndDuration struct {
		SbsPriceMplx    int64 `pg:"sbs_price_mplx" json:"sbs_price_mplx"`
		SbsDurationDays int64 `pg:"sbs_period_days" json:"sbs_period_days"`
	}

	UserCountByPlan struct {
		SubscriptionId int8  `pg:"subscription"`
		Count          int64 `pg:"users_count"`
	}

	UsersSnapshot struct {
		Day                   time.Time `pg:"urs_day" json:"day"`
		ProSubscriptions      int64     `pg:"urs_pro_subscriptions" json:"pro_subscriptions"`
		AdvancedSubscriptions int64     `pg:"urs_advanced_subscriptions" json:"advanced_subscriptions"`
		PayAsYouGo            int64     `pg:"urs_pay_as_you_go" json:"pay_as_you_go"`
		ActiveUsers           int64     `pg:"urs_active_users" json:"active_users"`
		TotalUsers            int64     `pg:"urs_total_users" json:"total_users"`
	}

	UsrIDs struct {
		DBId      int64
		DynamicId string
	}
)

// ClickHouse models
type (
	DailyAggregatedRequests struct {
		RequestType  string `json:"request_type"`
		Chain        string `json:"chain"`
		RequestPrice int64  `json:"price_per_request"`
		RequestCount int64  `json:"num_of_requests"`
	}

	DailyProvidersStat struct {
		Provider     string `json:"provider"`
		RequestType  string `json:"request_type"`
		Chain        string `json:"chain"`
		RequestCount int64  `json:"num_of_requests"`
	}
)
