package util

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func GracefulStop(waitGroup *sync.WaitGroup, waitTimeout time.Duration, stopFunc func()) {
	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT)
	<-gracefulStop

	// Run
	log.Infof("Received sigterm, stopping services")
	stopFunc()

	if waitGroup != nil {
		closeChan := make(chan struct{})

		go func() {
			defer close(closeChan)
			waitGroup.Wait()
		}()

		select {
		case <-closeChan:
			log.Info("Service stopped")
		case <-time.After(waitTimeout):
			log.Warnf("Service stopped after timeout")
		}
	}
}

func AsyncRunWithInterval(ctx context.Context, wg *sync.WaitGroup, interval time.Duration, runOnStart, runOnCtxDone bool, fn func(context.Context) error) error {
	if wg != nil {
		wg.Add(1)
		defer wg.Done()
	}
	if runOnStart {
		err := fn(ctx)
		if err != nil {
			return err
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				if runOnCtxDone {
					_ = fn(ctx)
				}
				return
			case <-time.After(interval):
				_ = fn(ctx)
			}
		}
	}()

	return nil
}
