package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/pkg/util"

	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
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
)

type (
	User struct {
		MplxBalance  int64                 `pg:"usr_mplx_balance" json:"mplx_balance"`
		DynamicID    string                `pg:"usr_dynamic_id" json:"dynamic_id"`
		CreatedAt    time.Time             `pg:"usr_created_at" json:"created_at"`
		Subscription postgres.Subscription `json:"subscription"`
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

func (u *User) FromDBModel(user *postgres.UserWithSubscription) {
	u.DynamicID = user.DynamicID
	u.MplxBalance = user.MplxBalance
	u.CreatedAt = user.User.CreatedAt
	u.Subscription = user.Subscription
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
