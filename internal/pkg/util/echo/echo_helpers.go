package echo

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	gommonLog "github.com/labstack/gommon/log"

	"aura-api/internal/pkg/log"
)

const (
	apiIdleTimeout     = time.Minute
	apiReadTimeout     = 5 * time.Second
	APIWriteTimeout    = 91 * time.Second
	errTimeout         = "Request Timeout"
	TokenParamName     = "token"     // located in path
	RestPathParamName  = "rest_path" // located in path
	ProxyPathWithToken = "/:token"
)

func InitBaseMiddlewares(router *echo.Echo, corsMiddleware echo.MiddlewareFunc) {
	router.HTTPErrorHandler = defaultHTTPErrorHandler
	router.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll: true,
		LogErrorFunc:    LogPanic,
	}))
	if corsMiddleware != nil {
		router.Use(corsMiddleware)
	}
	router.Use(middleware.BodyLimit("1M"))
	router.Use(CustomContextMiddleware)
}

func NewRateLimiter(skipper middleware.Skipper) echo.MiddlewareFunc {
	config := DefaultRateLimiterConfig
	config.Store = NewRateLimiterMemoryStoreWithConfig()
	config.Skipper = skipper
	config.IdentifierExtractor = func(c *CustomContext) (string, error) {
		// TODO: use API-token
		return c.RealIP(), nil
	}

	return RateLimiterWithConfig(config)
}

func SetupServer(router *echo.Echo, skipWriteTimeout bool) {
	router.DisableHTTP2 = true
	router.Logger.SetLevel(gommonLog.OFF)

	for _, s := range []*http.Server{router.Server, router.TLSServer} {
		s.ReadTimeout = apiReadTimeout
		s.IdleTimeout = apiIdleTimeout
		if !skipWriteTimeout {
			s.WriteTimeout = APIWriteTimeout
		}
	}
}

func LogPanic(_ echo.Context, err error, stack []byte) error {
	log.Logger.API.Errorf("PANIC RECOVER: %s %s", err, strconv.Quote(string(stack)))
	return nil
}

func defaultHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	if errors.Is(c.Request().Context().Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		err = c.String(http.StatusRequestTimeout, errTimeout)
		if err != nil {
			log.Logger.API.Errorf("defaultHTTPErrorHandler: string: %s", err)
		}

		return
	}

	var he *echo.HTTPError
	if !errors.As(err, &he) {
		he = echo.NewHTTPError(http.StatusInternalServerError)
	}

	// Issue #1426
	if m, ok := he.Message.(string); ok {
		he.Message = echo.Map{"message": m}
	}

	// Send response
	if c.Request().Method == http.MethodHead { // Issue #608
		err = c.NoContent(he.Code)
	} else {
		err = c.JSON(he.Code, he.Message)
	}
	if err != nil {
		log.Logger.API.Errorf("defaultHTTPErrorHandler: %s", err)
	}
}

var possibleContentTypes = map[string]struct{}{
	echo.MIMEApplicationJSON:          {},
	"application/json; charset=utf-8": {}, // echo.MIMEApplicationJSONCharsetUTF8 with capitalized part
	"application/json;charset=utf-8":  {},
}

func IsContentTypeValid(contentTypeHeader string) bool {
	_, ok := possibleContentTypes[strings.ToLower(contentTypeHeader)]

	return ok
}

func PrepareDomainForRefererHeader(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(domain, "http://"), "https://"), "/"))
}
