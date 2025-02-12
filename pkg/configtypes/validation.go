package configtypes

import (
	"errors"
	"fmt"
)

var ErrInvalidPort = errors.New("invalid port")

func (a APIConfig) Validate() error { //nolint:gocritic
	if a.Port == 0 {
		return fmt.Errorf("port: %s", ErrInvalidPort)
	}
	if a.GRPCPort == 0 {
		return fmt.Errorf("grpc: %s", ErrInvalidPort)
	}
	if a.SwaggerPort == 0 {
		return fmt.Errorf("swagger: %s", ErrInvalidPort)
	}
	if a.EmailToken == "" {
		return errors.New("invalid email token")
	}

	return nil
}

func (c ClickhouseConfig) Validate() error {
	if c.DSN == "" {
		return errors.New("invalid dsn")
	}

	if c.MigrationsPath == "" {
		return errors.New("invalid migrations path")
	}

	return nil
}

func (p PostgresConfig) Validate() error { //nolint:gocritic
	if p.URL == "" {
		return errors.New("invalid url")
	}

	return nil
}
