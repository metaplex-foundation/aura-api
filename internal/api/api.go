package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	consulAPI "github.com/hashicorp/consul/api"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/patrickmn/go-cache"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/swaggo/swag"

	//nolint:goimports
	"google.golang.org/grpc"

	"github.com/shopspring/decimal"

	"github.com/adm-metaex/aura-api/internal/api/config"
	"github.com/adm-metaex/aura-api/internal/api/docs"
	"github.com/adm-metaex/aura-api/internal/api/middlewares"
	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/configtypes"
	"github.com/adm-metaex/aura-api/pkg/dynamic"
	"github.com/adm-metaex/aura-api/pkg/email"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/proto"
	"github.com/adm-metaex/aura-api/pkg/stats"
	echo2 "github.com/adm-metaex/aura-api/pkg/util/echo"
)

type api struct { //nolint:govet // aligned to 176 bytes
	conf         configtypes.APIConfig
	certData     []byte
	router       *echo.Echo
	routerAPIDoc *echo.Echo
	waitGroup    *sync.WaitGroup
	ctx          context.Context
	ctxCancel    context.CancelFunc

	pgStorage   postgres.Storage
	chStorage   clickhouse.Storage
	cache       *cache.Cache
	emailSender email.Sender

	grpcServer *grpc.Server
	consulKV   *consulAPI.KV

	availableNetworks map[string]int64

	pricing          configtypes.PricingPlans
	mplxPrice        decimal.Decimal
	paymentRecipient solana.PublicKey
	paymentWatcher   paymentsWatcher

	statsCollector stats.Collector
}

const (
	cacheTTL              = 5 * time.Minute
	serverShutdownTimeout = time.Second * 5
)

func NewAPI(mainCtx context.Context, cfg config.Config) (a *api, err error) { //nolint:gocritic
	// increase uuid generation productivity
	uuid.EnableRandPool()

	ctx, cancelFunc := context.WithCancel(mainCtx)
	defer func() {
		if err != nil {
			cancelFunc()
		}
	}()

	pgStorage, err := postgres.New(mainCtx, cfg.PG)
	if err != nil {
		return nil, fmt.Errorf("PG storage init: %s", err)
	}
	chStorage, err := clickhouse.NewAndMigrate(cfg.CH, cfg.API.Hostname)
	if err != nil {
		return nil, fmt.Errorf("CH storage init: %s", err)
	}
	//panic(chStorage.InsertMockData(100000))

	emailSender := email.NewEmailSender(cfg.API.EmailToken)
	cacheInstance := cache.New(cacheTTL, cacheTTL)
	if err != nil {
		return a, fmt.Errorf("NewSubscriptionManager: %s", err)
	}
	availableNetworks, err := pgStorage.GetAvailableNetworks(ctx)
	if err != nil {
		return a, fmt.Errorf("GetAvailableNetworks: %s", err)
	}
	consulClient, err := consulAPI.NewClient(consulAPI.DefaultConfig())
	if err != nil {
		return a, fmt.Errorf("consulAPI.NewClient: %s", err)
	}
	consulKV := consulClient.KV()
	pair, _, err := consulKV.Get(configtypes.ConsulPricingPath, nil)
	if err != nil {
		return a, fmt.Errorf("consulKV.Get %s: %s", configtypes.ConsulPricingPath, err)
	}
	var pricing configtypes.PricingPlans
	if err = json.Unmarshal(pair.Value, &pricing); err != nil {
		return a, fmt.Errorf("PricingConfig: json.Unmarshal: %s", err)
	}
	pair, _, err = consulKV.Get(configtypes.ConsulMplxPricePath, nil)
	if err != nil {
		return a, fmt.Errorf("consulKV.Get %s: %s", configtypes.ConsulMplxPricePath, err)
	}
	price, err := decimal.NewFromString(string(pair.Value))
	if err != nil {
		return a, fmt.Errorf("NewFromString: %s", err)
	}
	pair, _, err = consulKV.Get(configtypes.ConsulPaymentsRecipientPath, nil)
	if err != nil {
		return a, fmt.Errorf("consulKV.Get %s: %s", configtypes.ConsulPaymentsRecipientPath, err)
	}
	if len(pair.Value) == 0 {
		return a, fmt.Errorf("empty value for key: %s", configtypes.ConsulPaymentsRecipientPath)
	}
	paymentRecepient, err := solana.PublicKeyFromBase58(string(pair.Value))
	if err != nil {
		return a, fmt.Errorf("PublicKeyFromBase58: %s", err)
	}
	paymentWatcher, err := newPaymentsWatcher(cfg.API.RPCAddress, &pgStorage, paymentRecepient)
	if err != nil {
		return a, fmt.Errorf("newPaymentsWatcher: %s", err)
	}

	statsCollector := stats.New(pgStorage, chStorage, *consulClient)

	// TODO: add consul watching
	g := grpc.NewServer()
	proto.RegisterAuraServer(g, &auraServer{
		pgStorage: &pgStorage,
		chStorage: chStorage,
		pricing:   pricing,
		mplxPrice: price,
	})
	a = &api{
		conf:         cfg.API,
		router:       initAPIServer(),
		routerAPIDoc: initAPIServer(),
		waitGroup:    &sync.WaitGroup{},
		ctx:          ctx,
		ctxCancel:    cancelFunc,

		pgStorage: pgStorage,
		chStorage: chStorage,
		cache:     cacheInstance,

		grpcServer: g,

		emailSender:       emailSender,
		availableNetworks: availableNetworks,
		consulKV:          consulKV,
		pricing:           pricing,
		mplxPrice:         price,
		paymentRecipient:  paymentRecepient,
		paymentWatcher:    paymentWatcher,

		statsCollector: statsCollector,
	}
	if cfg.API.CertFile != "" {
		a.certData, err = os.ReadFile(cfg.API.CertFile)
		if err != nil {
			return nil, fmt.Errorf("fail to read certificate (%s): %s", cfg.API.CertFile, err)
		}
	}

	dynamicClient, err := dynamic.NewDynamicClient(cfg.API.DynamicAPIToken, cfg.API.DynamicEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("NewDynamicClient: %w", err)
	}
	authMiddleware, err := middlewares.NewAuthMiddleware(ctx, cfg.API.DynamicJWKSEndpoint, dynamicClient)
	if err != nil {
		return nil, fmt.Errorf("NewAuthMiddleware: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname: %w", err)
	}
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%d", hostname, cfg.API.Port)
	swag.Register(docs.SwaggerInfo.InstanceName(), docs.SwaggerInfo)

	a.initAPIHandlers(authMiddleware)
	a.initAPIDocsHandlers()

	err = chStorage.RunInitialAggregation(ctx)
	if err != nil {
		return nil, fmt.Errorf("RunInitialAggregation: %s", err)
	}
	go chStorage.RunStatsAggregator(ctx)
	go a.statsCollector.RunStatsCollector(ctx)
	go a.listenConsul(ctx)
	// TODO: consider consul
	if cfg.API.IsFrontendAPI {
		go paymentWatcher.watchPayments(mainCtx)
		go paymentWatcher.cancelUnpaidPayments(mainCtx)
	}

	return a, nil
}

func initAPIServer() *echo.Echo {
	s := echo.New()
	echo2.SetupServer(s, false)

	echo2.InitBaseMiddlewares(s, middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	s.Use(echo2.RequestTimeoutMiddleware(nil))

	return s
}

// @title						Swagger Aura
// @version					0.0.3
// @description				Swagger API server for Aura API.
// @termsOfService				http://swagger.io/terms/
// @BasePath					/
// @schemes					http https
// @accept						json
//
// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description				Bearer JWT
func (a *api) initAPIDocsHandlers() {
	// api docs
	a.routerAPIDoc.GET("*", echoSwagger.WrapHandler)
}

func (a *api) initAPIHandlers(authMiddleware *middlewares.AuthMiddleware) {
	log.Logger.API.Infof("initAPIHandlers")

	// public
	a.router.GET("/networks", a.getSupportedNetworksHandler)
	a.router.GET("/plans", a.getSubscriptionPlans)
	// protected
	authMW := authMiddleware.LoadUser()
	protectedGroup := a.router.Group("", authMW)
	protectedGroup.GET("/user", a.getUserHandler)
	protectedGroup.PATCH("/plan", a.updateSubscriptionPlan)
	// API keys
	apiKeysGroup := protectedGroup.Group("/keys")
	apiKeysGroup.GET("", a.apiKeysHandler)
	apiKeysGroup.GET("/:token", a.apiKeyHandler)
	apiKeysGroup.POST("", a.createAPIKeyHandler)
	apiKeysGroup.PATCH("/:token", a.updateAPIKeyHandler)
	apiKeysGroup.DELETE("/:token", a.deleteAPIKeyHandler)
	// User stats
	statsGroup := protectedGroup.Group("/stats")
	statsGroup.GET("/response/time", a.getAPIResponseTimes)
	statsGroup.GET("/request/volume", a.getAPIRequestsVolume)
	statsGroup.GET("/credits/usage", a.getAPICreditsUsage)
	// Payments
	paymentGroup := protectedGroup.Group("/payments")
	paymentGroup.GET("/link", a.getPaymentLink)
	paymentGroup.GET("/status", a.getPaymentStatus)
	paymentGroup.GET("/history", a.getPaymentHistory)
}

func (a *api) Run() (err error) {
	addr := fmt.Sprintf(":%d", a.conf.Port)
	if len(a.certData) != 0 {
		err = a.router.StartTLS(addr, a.certData, a.certData)
	} else {
		err = a.router.Start(addr)
	}
	if err != http.ErrServerClosed { //nolint:errorlint
		return err
	}

	return nil
}

func (a *api) RunGRPC() error {
	log.Logger.API.Infof("grpc server listening at :%d", a.conf.GRPCPort)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.conf.GRPCPort))
	if err != nil {
		return err
	}

	return a.grpcServer.Serve(lis)
}

func (a *api) RunAPIDoc() (err error) {
	addr := fmt.Sprintf(":%d", a.conf.SwaggerPort)
	if len(a.certData) != 0 {
		err = a.routerAPIDoc.StartTLS(addr, a.certData, a.certData)
	} else {
		err = a.routerAPIDoc.Start(addr)
	}
	if err != http.ErrServerClosed { //nolint:errorlint
		return err
	}

	return nil
}

func (a *api) Stop() error {
	ctx, cancel := context.WithTimeout(a.ctx, serverShutdownTimeout)
	defer cancel()

	go a.routerAPIDoc.Shutdown(ctx) //nolint:errcheck
	err := a.router.Shutdown(ctx)
	if err != nil {
		log.Logger.API.Errorf("router.Shutdown: %s", err)
	}
	a.grpcServer.Stop()

	a.ctxCancel()

	return nil
}

func (a *api) WaitGroup() *sync.WaitGroup {
	return a.waitGroup
}
