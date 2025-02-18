package configtypes

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/shopspring/decimal"
)

// struct field names are used for env variable names. Edit with care
type (
	APIConfig struct {
		Hostname             string `envconfig:"HOSTNAME" default:"unspecified" required:"true" split_words:"true"`
		CertFile             string `required:"false" split_words:"true"`
		EmailToken           string `required:"true" split_words:"true"`
		DynamicEnvironmentID string `required:"true" split_words:"true"`
		DynamicJWKSEndpoint  string `envconfig:"API_DYNAMIC_JWKS_ENDPOINT" required:"true" split_words:"true"`
		DynamicAPIToken      string `envconfig:"API_DYNAMIC_API_TOKEN" required:"true" split_words:"true"`
		RPCAddress           string `envconfig:"API_RPC_ADDRESS" required:"true" split_words:"true"`

		Port        uint64 `required:"true" split_words:"true"`
		GRPCPort    uint64 `required:"true" split_words:"true"`
		SwaggerPort uint64 `required:"true" split_words:"true"`

		IsFrontendAPI bool `envconfig:"API_IS_FRONTEND_API" required:"true" split_words:"true"`
	}
)

// struct field names are used for env variable names. Edit with care
type (
	PostgresConfig struct {
		URL            string `required:"true" split_words:"true"`
		MigrationsPath string `required:"true" split_words:"true"`
	}
	ClickhouseConfig struct {
		DSN            string `required:"true" split_words:"true"`
		MigrationsPath string `required:"true" split_words:"true"`
	}
	ServiceConfig struct {
		Name  string `envconfig:"NAME" default:"unspecified" required:"false"`
		Level string `envconfig:"LEVEL" required:"false"`
	}
)

type (
	PricingModel struct {
		RequestsPerSecond int32           `json:"requests_per_second"`
		PriceUSD          decimal.Decimal `json:"price_usd"`
	}
	PricingConfig struct {
		SolanaDAS                 PricingModel `json:"solana_das"`
		EclipseDAS                PricingModel `json:"eclipse_das"`
		SolanaRPC                 PricingModel `json:"solana_rpc"`
		EclipseRPC                PricingModel `json:"eclipse_rpc"`
		SolanaGetProgramAccounts  PricingModel `json:"solana_get_program_accounts"`
		EclipseGetProgramAccounts PricingModel `json:"eclipse_get_program_accounts"`
		SolanaSWQOS               PricingModel `json:"solana_swqos"`
		EclipseSWQOS              PricingModel `json:"eclipse_swqos"`
		SolanaWebsocket           PricingModel `json:"solana_websocket"`
		EclipseWebsocket          PricingModel `json:"eclipse_websocket"`
		APITokensLimit            uint64       `json:"api_tokens_limit"`
		MonthlyPriceMPLX          *int64       `json:"monthly_price_mplx"`
		PrioritySupport           bool         `json:"priority_support"`
	}
	PricingPlans struct {
		Free      PricingConfig `json:"free"`
		Developer PricingConfig `json:"developer"`
		Advanced  PricingConfig `json:"advanced"`
		Pro       PricingConfig `json:"pro"`
	}
)

const (
	ConsulPricingPath           = "config/aura-api/pricing"
	ConsulMplxPricePath         = "config/aura-api/mplx"
	ConsulPaymentsRecipientPath = "config/aura-api/payments/recipient"
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
