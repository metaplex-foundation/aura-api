package util

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/internal/pkg/types"
)

func ErrMsg(err error) string {
	if err == nil {
		return ""
	}

	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr == nil {
		return err.Error()
	}
	rpcResponse, ok := httpErr.Message.(*types.RPCResponse)
	if !ok || rpcResponse == nil || rpcResponse.Error == nil {
		return httpErr.Error()
	}

	return rpcResponse.Error.Message
}

var (
	AuraNoAvailableTargetsErrorResponse = types.NewRPCErrorResponse(types.NewRPCError(2000, "No available targets", nil), nil)
	AuraAttemptsExceededErrorResponse   = types.NewRPCErrorResponse(types.NewRPCError(2001, "Attempts exceeded", nil), nil)
	ErrChainNotSupported                = types.NewRPCErrorResponse(types.NewRPCError(2002, "Chain not supported", nil), nil)
)

var ErrBadStatusCode = errors.New("bad status code")

var ErrTokenInvalid = echo.NewHTTPError(http.StatusUnauthorized, "invalid api token")
