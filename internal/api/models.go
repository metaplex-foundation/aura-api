package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
)

type (
	CreateAPIKeyRequestParams struct {
		Name     string   `json:"name"`
		Networks []string `json:"networks" enums:"aura, solana"`
	}
	UpdateAPIKeyRequestParams struct {
		Name     *string  `json:"name" extensions:"x-nullable"`
		Networks []string `json:"networks" enums:"aura, solana"`
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
