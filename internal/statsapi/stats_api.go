package statsapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/adm-metaex/aura-api/internal/config"
	"github.com/adm-metaex/aura-api/internal/statsapi/docs"
	"github.com/adm-metaex/aura-api/internal/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/configtypes"
	"github.com/adm-metaex/aura-api/pkg/log"
	echo2 "github.com/adm-metaex/aura-api/pkg/util/echo"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/swaggo/swag"
)

type statsApi struct { //nolint:govet
	conf         configtypes.StatsAPIConfig
	certData     []byte
	router       *echo.Echo
	routerAPIDoc *echo.Echo
	waitGroup    *sync.WaitGroup
	ctx          context.Context
	ctxCancel    context.CancelFunc

	pgStorage postgres.Storage
	chStorage clickhouse.Storage
}

const (
	serverShutdownTimeout = time.Second * 5
)

func NewAPI(mainCtx context.Context, cfg config.StatsAPIConfig) (a *statsApi, err error) { //nolint:gocritic
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

	a = &statsApi{
		conf:         cfg.API,
		router:       initAPIServer(cfg.API.AllowedOrigins),
		routerAPIDoc: initAPIServer(cfg.API.AllowedOrigins),
		waitGroup:    &sync.WaitGroup{},
		ctx:          ctx,
		ctxCancel:    cancelFunc,

		pgStorage: pgStorage,
		chStorage: chStorage,
	}
	if cfg.API.CertFile != "" {
		a.certData, err = os.ReadFile(cfg.API.CertFile)
		if err != nil {
			return nil, fmt.Errorf("fail to read certificate (%s): %s", cfg.API.CertFile, err)
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname: %w", err)
	}
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%d", hostname, cfg.API.Port)
	swag.Register(docs.SwaggerInfo.InstanceName(), docs.SwaggerInfo)

	a.initAPIHandlers()
	a.initAPIDocsHandlers()

	return a, nil
}

func initAPIServer(allowedOrigins []string) *echo.Echo {
	s := echo.New()
	echo2.SetupServer(s, false)

	echo2.InitBaseMiddlewares(s, middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))
	s.Use(echo2.RequestTimeoutMiddleware(nil))

	return s
}

// @title						Swagger Aura
// @version					    0.0.3
// @description				    Swagger API server for Aura Stats API.
// @termsOfService				http://swagger.io/terms/
// @BasePath					/
// @schemes					    http https
// @accept						json
func (a *statsApi) initAPIDocsHandlers() {
	// api docs
	a.routerAPIDoc.GET("*", echoSwagger.WrapHandler)
}

func (a *statsApi) initAPIHandlers() {
	log.Logger.StatsAPI.Infof("initAPIHandlers")

	a.router.GET("/ping", a.ping)

	a.router.GET("/network/revenue/paid/total", a.getNetworkRevenuePaidTotal)
	a.router.GET("/network/revenue/paid/daily", a.getNetworkRevenuePaidDaily)

	a.router.GET("/network/revenue/distributed/total", a.getNetworkRevenueDistributedTotal)
	a.router.GET("/network/revenue/distributed/daily", a.getNetworkRevenueDistributedDaily)

	a.router.GET("/network/revenue/earned/total", a.getTotalRewardsEarned)
	a.router.GET("/network/revenue/earned/daily", a.getDailyRewardsEarned)

	a.router.GET("/metrics/requests/daily", a.getDailyRequests)
	a.router.GET("/metrics/users/daily", a.getDailyUsersSnapshot)
}

func (a *statsApi) Run() (err error) {
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

func (a *statsApi) RunAPIDoc() (err error) {
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

func (a *statsApi) Stop() error {
	ctx, cancel := context.WithTimeout(a.ctx, serverShutdownTimeout)
	defer cancel()

	go a.routerAPIDoc.Shutdown(ctx) //nolint:errcheck
	err := a.router.Shutdown(ctx)
	if err != nil {
		log.Logger.StatsAPI.Errorf("router.Shutdown: %s", err)
	}

	a.ctxCancel()

	return nil
}

func (a *statsApi) WaitGroup() *sync.WaitGroup {
	return a.waitGroup
}
