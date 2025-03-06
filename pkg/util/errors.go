package util

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/pkg/types"
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
	ErrChainNotSupported                = types.NewRPCErrorResponse(types.NewRPCError(2002, "Network not supported", nil), nil)
)

var ErrBadStatusCode = errors.New("bad status code")

var ErrTokenInvalid = echo.NewHTTPError(http.StatusUnauthorized, "invalid api token")

func ToHTTPError(err error) *echo.HTTPError {
	if err == nil {
		return nil
	}

	switch err.(type) {
	case *SubscriptionUpdatedTooOftenError:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case *PgTransactionError:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	case *PgSelectError:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case *PgInsertError:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	case *PgUpdateError:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	case *UpgradeSubscriptionError:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case *DowngradeSubscriptionError:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "unexpected error")
	}
}

type SubscriptionUpdatedTooOftenError struct{}

func (e *SubscriptionUpdatedTooOftenError) Error() string {
	return "cannot update the subscription plan more than once a day"
}

type PgTransactionError struct{ Msg string }

func (te *PgTransactionError) Error() string {
	return te.Msg
}

type PgSelectError struct{ Msg string }

func (sne *PgSelectError) Error() string {
	return sne.Msg
}

type PgInsertError struct{ Msg string }

func (pie *PgInsertError) Error() string {
	return pie.Msg
}

type PgUpdateError struct{ Msg string }

func (pue *PgUpdateError) Error() string {
	return pue.Msg
}

type UpgradeSubscriptionError struct{ Msg string }

func (use *UpgradeSubscriptionError) Error() string {
	return use.Msg
}

type DowngradeSubscriptionError struct{ Msg string }

func (dse *DowngradeSubscriptionError) Error() string {
	return dse.Msg
}
