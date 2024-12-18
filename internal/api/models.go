package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/util"

	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
)

const (
	freeSubcriptionPlanName      = "Free"
	developerSubcriptionPlanName = "Developer"
	advancedSubcriptionPlanName  = "Advanced"
	proSubcriptionPlanName       = "Pro"

	metaplexTokenDecimals = 6
)

var (
	allowedResponseTimeHistoryGranularity = map[string]struct{}{
		clickhouse.HourlyGranularity: {},
		clickhouse.DailyGranularity:  {},
	}
	allowedTimeframes = map[string]time.Duration{
		"1h":  time.Hour,
		"4h":  4 * time.Hour,
		"12h": 12 * time.Hour,
		"1d":  24 * time.Hour,
		"7d":  7 * 24 * time.Hour,
		"14d": 14 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
	}

	metaplexTokenDecimalsMultiplier = decimal.NewFromFloat(10).Pow(decimal.NewFromFloat(metaplexTokenDecimals))
)

type (
	CreateAPIKeyRequestParams struct {
		Name     string   `json:"name"`
		Networks []string `json:"networks" enums:"Aura, Solana"`
	}
	UpdateAPIKeyRequestParams struct {
		Name     *string  `json:"name" extensions:"x-nullable"`
		Networks []string `json:"networks" enums:"Aura, Solana"`
	}
	StatsRequestParams struct {
		Granularity string
		StartTime   time.Time
		TokenUUID   *uuid.UUID
		Network     *string
		RPCMethod   *string
	}
	UpdateSubscriptionParams struct {
		SubscriptionID int64 `json:"subscription_id"`
	}
)

type (
	User struct {
		MplxBalance       int64                   `json:"mplx_balance"`
		DynamicID         string                  `json:"dynamic_id"`
		CreatedAt         time.Time               `json:"created_at"`
		LastUpdatedPlanAt time.Time               `json:"last_updated_plan_at"`
		Subscription      SubscriptionWithPricing `json:"subscription"`
	}
	UIPricing struct {
		RequestsPerSecond int   `json:"requests_per_second"`
		PriceMPLX         int64 `json:"price_mplx"`
	}
	Pricing struct {
		AuraDAS            UIPricing `json:"aura_das"`
		EclipseDAS         UIPricing `json:"eclipse_das"`
		EclipseRPC         UIPricing `json:"eclipse_rpc"`
		SolanaRPC          UIPricing `json:"solana_rpc"`
		GetProgramAccounts UIPricing `json:"get_program_accounts"`
		SolanaSWQOS        UIPricing `json:"solana_swqos"`
		Websocket          UIPricing `json:"websocket"`
		MonthlyPriceMPLX   *int64    `json:"monthly_price_mplx"`
	}
	SubscriptionWithPricing struct {
		ID              int64   `json:"id"`
		Name            string  `json:"name"`
		Priority        int64   `json:"priority"`
		APITokensLimit  int64   `json:"api_tokens_limit"`
		PrioritySupport bool    `json:"priority_support"`
		Pricing         Pricing `json:"pricing"`
	}
)

func (p *CreateAPIKeyRequestParams) Validate(availableNetworks map[string]int64) error {
	if p.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Empty API key name")
	}
	if len(p.Networks) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Neither network selected")
	}
	for _, network := range p.Networks {
		if _, ok := availableNetworks[network]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Selected invalid network: %s", network))
		}
	}

	return nil
}

func (p *UpdateAPIKeyRequestParams) Validate(availableNetworks map[string]int64) error {
	if p.Name != nil && *p.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Empty API key name")
	}
	for _, network := range p.Networks {
		if _, ok := availableNetworks[network]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Selected invalid network: %s", network))
		}
	}

	return nil
}

func (a *api) UserWithCurrentPlanFromDBModel(user *postgres.UserWithCurrentPlan) (u User) {
	u.DynamicID = user.DynamicID
	u.MplxBalance = user.MplxBalance
	u.CreatedAt = user.User.CreatedAt
	u.LastUpdatedPlanAt = user.User.LastUpdatedPlanAt
	u.Subscription = a.SubscriptionWithPricingFromDBModel(user.Plan)
	return u
}

func (s *StatsRequestParams) Bind(c echo.Context, availableNetworks map[string]int64) (err error) {
	s.Granularity = c.QueryParam(granularityParam)
	if s.Granularity == "" {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Missed required param: %s", granularityParam))
	}
	if _, ok := allowedResponseTimeHistoryGranularity[s.Granularity]; !ok {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid granularity: %s. Allowed: %s", s.Granularity, util.MapKeys(allowedResponseTimeHistoryGranularity)))
	}

	timeframeParamString := c.QueryParam(timeframeParam)
	if timeframeParamString == "" {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Missed required param: %s", timeframeParam))
	}
	s.StartTime, err = getTimeInterval(timeframeParamString)
	if err != nil {
		return err
	}
	if time.Since(s.StartTime) < 24*time.Hour && s.Granularity == clickhouse.DailyGranularity {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Need to select more detailed granularity for selected timeframe: %s", timeframeParamString))
	}

	if tokenParamString := c.QueryParam(tokenParam); tokenParamString != "" {
		t, err := uuid.Parse(tokenParamString)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, tokenParam) // TODO
		}
		s.TokenUUID = &t
	}
	if networkParamString := c.QueryParam(networkParam); networkParamString != "" {
		// TODO: refactor
		networkParamString = strings.Title(strings.ToLower(networkParamString))
		_, ok := availableNetworks[networkParamString]
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Selected invalid network: %s", networkParamString))
		}
		lowerCaseNetwork := strings.ToLower(networkParamString)
		s.Network = &lowerCaseNetwork
	}
	if methodParamString := c.QueryParam(methodParam); methodParamString != "" {
		if s.Network == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Required to select network when selecting rpc_method")
		}
		s.RPCMethod = &methodParamString
	}

	return nil
}

func getTimeInterval(timeframe string) (time.Time, error) {
	timeframeDuration, ok := allowedTimeframes[timeframe]
	if !ok {
		return time.Time{}, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported timeframe: %s. Allowed: %s", timeframe, util.MapKeys(allowedTimeframes)))
	}

	return time.Now().UTC().Add(-timeframeDuration), nil
}

func (a *api) SubscriptionWithPricingFromDBModel(plan postgres.Plan) SubscriptionWithPricing {
	subscriptionWithPricing := SubscriptionWithPricing{
		ID:             plan.PlanID,
		Name:           plan.Name,
		Priority:       plan.Priority,
		APITokensLimit: plan.TokenLimit,
	}
	switch plan.Name {
	case freeSubcriptionPlanName:
		subscriptionWithPricing.Pricing = a.ConvertUIPricing(a.pricing.Free)
		subscriptionWithPricing.PrioritySupport = a.pricing.Free.PrioritySupport
	case developerSubcriptionPlanName:
		subscriptionWithPricing.Pricing = a.ConvertUIPricing(a.pricing.Developer)
		subscriptionWithPricing.PrioritySupport = a.pricing.Developer.PrioritySupport
	case advancedSubcriptionPlanName:
		subscriptionWithPricing.Pricing = a.ConvertUIPricing(a.pricing.Advanced)
		subscriptionWithPricing.PrioritySupport = a.pricing.Advanced.PrioritySupport
	case proSubcriptionPlanName:
		subscriptionWithPricing.Pricing = a.ConvertUIPricing(a.pricing.Pro)
		subscriptionWithPricing.PrioritySupport = a.pricing.Pro.PrioritySupport
	default:
		log.Logger.API.Errorf("invalid subscription name: %s", plan.Name)
	}

	return subscriptionWithPricing
}

func (a *api) getSubscriptionsWithPricingList(subscriptions []postgres.Plan) []SubscriptionWithPricing {
	result := make([]SubscriptionWithPricing, 0, len(subscriptions))
	for _, s := range subscriptions {
		subscriptionWithPricing := a.SubscriptionWithPricingFromDBModel(s)
		if subscriptionWithPricing.APITokensLimit != 0 {
			result = append(result, subscriptionWithPricing)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})

	return result
}

func (a *api) ConvertUIPricing(cfg PricingConfig) Pricing {
	return Pricing{
		AuraDAS:            a.UiPricingModel(cfg.AuraDAS),
		EclipseDAS:         a.UiPricingModel(cfg.EclipseDAS),
		EclipseRPC:         a.UiPricingModel(cfg.EclipseRPC),
		SolanaRPC:          a.UiPricingModel(cfg.SolanaRPC),
		GetProgramAccounts: a.UiPricingModel(cfg.GetProgramAccounts),
		SolanaSWQOS:        a.UiPricingModel(cfg.SolanaSWQOS),
		Websocket:          a.UiPricingModel(cfg.Websocket),
		MonthlyPriceMPLX:   cfg.MonthlyPriceMPLX,
	}
}

func (a *api) UiPricingModel(model PricingModel) UIPricing {
	return UIPricing{
		RequestsPerSecond: model.RequestsPerSecond,
		PriceMPLX:         model.PriceUSD.Div(a.mplxPrice).Truncate(metaplexTokenDecimals).Mul(metaplexTokenDecimalsMultiplier).Floor().BigInt().Int64(),
	}
}
