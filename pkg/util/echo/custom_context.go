package echo

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/adm-metaex/aura-api/pkg/dynamic"
	"github.com/adm-metaex/aura-api/pkg/util"
)

const (
	timeout = APIWriteTimeout - time.Second
)

type CustomContext struct {
	metrics     *util.RuntimeMetrics
	reqDuration time.Time
	dynamicUser *dynamic.User

	echo.Context
}

func CustomContextMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cc := &CustomContext{Context: c}
		cc.InitReqDuration()
		cc.InitMetrics()

		return next(cc)
	}
}

func RequestTimeoutMiddleware(skipper middleware.Skipper) echo.MiddlewareFunc {
	if skipper == nil {
		skipper = middleware.DefaultSkipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if skipper(c) {
				return next(c)
			}

			// hande timeout
			timeoutCtx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()
			c.SetRequest(c.Request().WithContext(timeoutCtx))

			return next(c)
		}
	}
}

func (c *CustomContext) InitReqDuration() {
	c.reqDuration = time.Now()
}
func (c *CustomContext) GetReqDuration() time.Time {
	return c.reqDuration
}

func (c *CustomContext) InitMetrics() {
	c.metrics = util.NewRuntimeMetrics()
}
func (c *CustomContext) GetMetrics() *util.RuntimeMetrics {
	return c.metrics
}
func (c *CustomContext) SetDynamicUser(u *dynamic.User) {
	c.dynamicUser = u
}
func (c *CustomContext) GetDynamicUser() *dynamic.User {
	return c.dynamicUser
}
