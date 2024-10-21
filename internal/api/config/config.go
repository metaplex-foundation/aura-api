package config

import (
	"fmt"

	"github.com/adm-metaex/aura-api/internal/pkg/configtypes"
)

type Config struct {
	CH  configtypes.ClickhouseConfig
	PG  configtypes.PostgresConfig
	API configtypes.APIConfig
}

func (c Config) Validate() error { //nolint:gocritic
	if err := c.API.Validate(); err != nil {
		return fmt.Errorf("api: %s", err)
	}
	if err := c.PG.Validate(); err != nil {
		return fmt.Errorf("postgres: %s", err)
	}
	if err := c.CH.Validate(); err != nil {
		return fmt.Errorf("clickhouse: %s", err)
	}

	return nil
}
