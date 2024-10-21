package configtypes

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// struct field names are used for env variable names. Edit with care
type (
	APIConfig struct {
		Hostname   string `envconfig:"HOSTNAME" default:"unspecified" required:"true" split_words:"true"`
		CertFile   string `required:"false" split_words:"true"`
		EmailToken string `required:"true" split_words:"true"`

		Port        uint64 `required:"true" split_words:"true"`
		GRPCPort    uint64 `required:"true" split_words:"true"`
		SwaggerPort uint64 `required:"true" split_words:"true"`
	}
)

// struct field names are used for env variable names. Edit with care
type (
	PostgresConfig struct {
		Host           string `required:"true" split_words:"true"`
		User           string `required:"true" split_words:"true"`
		Pass           string `required:"true" split_words:"true"`
		DB             string `required:"true" split_words:"true"`
		MigrationsPath string `required:"true" split_words:"true"`
		Port           uint64 `required:"true" split_words:"true"`
	}
	ClickhouseConfig struct {
		DSN string `required:"true" split_words:"true"`
	}
	ServiceConfig struct {
		Name  string `envconfig:"NAME" default:"unspecified" required:"false"`
		Level string `envconfig:"LEVEL" required:"false"`
	}
)

type PossibleConfig interface {
	Validate() error
}

func LoadFile[T PossibleConfig](envFile string) (c T, err error) {
	if envFile != "" {
		err = godotenv.Load(envFile)
		if err != nil {
			return c, fmt.Errorf("godotenv.Load (%s): %w", envFile, err)
		}
	}

	err = envconfig.Process("", &c)
	if err != nil {
		return c, err
	}

	err = c.Validate()
	if err != nil {
		return c, fmt.Errorf("validate: %s", err)
	}

	return c, nil
}
