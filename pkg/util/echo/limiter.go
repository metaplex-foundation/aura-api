package echo

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/adm-metaex/aura-api/pkg/log"
)

// RateLimiterStore is the interface to be implemented by custom stores.
type RateLimiterStore interface {
	// Stores for the rate limiter have to implement the Allow method
	Allow(identifier string, limit int64) (bool, error)
}

type (
	// RateLimiterConfig defines the configuration for the rate limiter
	RateLimiterConfig struct {
		Skipper middleware.Skipper
		// IdentifierExtractor uses echo.Context to extract the identifier for a visitor
		IdentifierExtractor Extractor
		// Store defines a store for the rate limiter
		Store RateLimiterStore
		// ErrorHandler provides a handler to be called when IdentifierExtractor returns an error
		ErrorHandler func(context echo.Context, err error) error
		// DenyHandler provides a handler to be called when RateLimiter denies access
		DenyHandler func(context echo.Context, identifier string, err error) error
	}

	Extractor func(context *CustomContext) (string, error)
)

var DefaultRateLimiterConfig = RateLimiterConfig{
	Skipper:      middleware.DefaultSkipper,
	ErrorHandler: middleware.DefaultRateLimiterConfig.ErrorHandler,
	DenyHandler:  middleware.DefaultRateLimiterConfig.DenyHandler,
}

func RateLimiterWithConfig(config RateLimiterConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultRateLimiterConfig.Skipper
	}
	if config.IdentifierExtractor == nil {
		log.Logger.General.Fatal("RateLimiterWithConfig: IdentifierExtractor must be provided")
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultRateLimiterConfig.ErrorHandler
	}
	if config.DenyHandler == nil {
		config.DenyHandler = DefaultRateLimiterConfig.DenyHandler
	}
	if config.Store == nil {
		log.Logger.General.Fatal("RateLimiterWithConfig: Store configuration must be provided")
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			cc, _ := c.(*CustomContext)
			identifier, err := config.IdentifierExtractor(cc)
			if err != nil {
				c.Error(config.ErrorHandler(c, err))
				return nil
			}
			// TODO
			if allow, err := config.Store.Allow(identifier, 30); !allow {
				c.Error(config.DenyHandler(c, identifier, err))
				return nil
			}

			return next(c)
		}
	}
}

type (
	// RateLimiterMemoryStore is the built-in store implementation for RateLimiter
	RateLimiterMemoryStore struct {
		visitors map[string]*Visitor
		timeNow  func() time.Time

		lastCleanup time.Time
		expiresIn   time.Duration

		mutex sync.Mutex
	}
	// Visitor signifies a unique user's limiter details
	Visitor struct {
		*rate.Limiter
		lastSeen time.Time
	}
)

func NewRateLimiterMemoryStoreWithConfig() (store *RateLimiterMemoryStore) {
	return &RateLimiterMemoryStore{
		visitors:    make(map[string]*Visitor),
		timeNow:     time.Now,
		lastCleanup: time.Now(),
		expiresIn:   middleware.DefaultRateLimiterMemoryStoreConfig.ExpiresIn,
		mutex:       sync.Mutex{},
	}
}

func (store *RateLimiterMemoryStore) Allow(identifier string, limit int64) (bool, error) {
	store.mutex.Lock()
	limiter, exists := store.visitors[identifier]
	if !exists || int64(limiter.Limit()) != limit {
		limiter = &Visitor{
			Limiter: rate.NewLimiter(rate.Limit(limit), int(limit)),
		}
		store.visitors[identifier] = limiter
	}
	limiter.lastSeen = store.timeNow()
	if limiter.lastSeen.Sub(store.lastCleanup) > store.expiresIn {
		store.cleanupStaleVisitors()
	}
	store.mutex.Unlock()

	return limiter.AllowN(store.timeNow(), 1), nil
}

func (store *RateLimiterMemoryStore) cleanupStaleVisitors() {
	now := store.timeNow()
	for id, visitor := range store.visitors {
		if now.Sub(visitor.lastSeen) > store.expiresIn {
			delete(store.visitors, id)
		}
	}
	store.lastCleanup = store.timeNow()
}
