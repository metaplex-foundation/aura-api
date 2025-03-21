package main

import (
	"context"
	"flag"
	"time"

	"github.com/adm-metaex/aura-api/internal/config"
	"github.com/adm-metaex/aura-api/internal/statsapi"
	"github.com/adm-metaex/aura-api/pkg/configtypes"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/util"
)

const (
	waitTimeout = time.Second * 5
)

type flags struct {
	logLevel string
	envFile  string
}

// Setup flags
func getFlags() (f flags) {
	flag.StringVar(&f.logLevel, "log", "info", "log level [debug|info|warn|error|crit]")
	flag.StringVar(&f.envFile, "envFile", "", "path to .env file")
	flag.Parse()

	return
}

func main() {
	f := getFlags()
	err := log.Setup(f.logLevel)
	if err != nil {
		log.Logger.StatsAPI.Fatalf("Log setup: %s", err)
	}

	cfg, err := configtypes.LoadFile[config.StatsAPIConfig](f.envFile)
	if err != nil {
		log.Logger.StatsAPI.Fatalf("Config: %s", err)
	}

	log.Logger.StatsAPI.Infof("Start service")

	mainCtx, cancelFn := context.WithCancel(context.Background())
	app, err := statsapi.NewAPI(mainCtx, cfg)
	if err != nil {
		log.Logger.StatsAPI.Fatalf("NewAPI: %s", err)
	}

	// API
	go func() {
		if err := app.Run(); err != nil {
			log.Logger.StatsAPI.Fatalf("Run: %s", err)
		}
	}()
	// API doc
	go func() {
		if err := app.RunAPIDoc(); err != nil {
			log.Logger.StatsAPI.Fatalf("RunAPIDoc: %s", err)
		}
	}()
	// Metrics
	go func() {
		if err := app.RunMetrics(); err != nil {
			log.Logger.API.Fatalf("Metrics: %s", err)
		}
	}()

	// Termination handler.
	util.GracefulStop(app.WaitGroup(), waitTimeout, func() {
		err = app.Stop()
		if err != nil {
			log.Logger.StatsAPI.Error(err.Error())
		}
		cancelFn()
	})
}
