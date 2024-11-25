package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
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

func (p *CreateAPIKeyRequestParams) Validate(availableNetworks map[string]int64) error {
	if p.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name")
	}
	if len(p.Networks) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "networks")
	}
	for _, network := range p.Networks {
		if _, ok := availableNetworks[network]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "networks")
		}
	}

	return nil
}

func (p *UpdateAPIKeyRequestParams) Validate(availableNetworks map[string]int64) error {
	if p.Name != nil && *p.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name")
	}
	for _, network := range p.Networks {
		if _, ok := availableNetworks[network]; !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "networks")
		}
	}

	return nil
}
