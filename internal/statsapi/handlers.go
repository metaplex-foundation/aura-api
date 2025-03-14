package statsapi

import (
	"net/http"

	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/labstack/echo/v4"
)

// ping godoc
//
//	@Summary		Live check
//	@Description	Endpoint is used to check if API is alive
//	@Tags			ping
//	@Produce		json
//	@Success		200	{string}	string
//	@Failure		400	{object}	error
//	@Failure		401	{object}	error
//	@Failure		500	{object}	error
//	@Router			/ping [get]
func (a *statsApi) ping(c echo.Context) (err error) {
	return c.JSON(http.StatusOK, "pong")
}

// getNetworkRevenuePaidTotal godoc
//
//	@Summary		Get total MPLX revenue
//	@Description	Returns total amount of MPLX paid by users to the gateway
//	@Tags			revenue
//	@Produce		json
//	@Success		200				{object}	TotalPaidMPLXResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/network/revenue/paid/total [get]
func (a *statsApi) getNetworkRevenuePaidTotal(c echo.Context) (err error) {
	totalPaidMPLX, err := a.pgStorage.GetTotalMPLXVolume(c.Request().Context())
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetTotalMPLXVolume: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, TotalPaidMPLXResponse{PaidTotal: totalPaidMPLX})
}

// getNetworkRevenuePaidDaily godoc
//
//	@Summary		Get MPLX revenue in a timeframe
//	@Description	Returns MPLX paid by users to the gateway in a requested timeframe
//	@Tags			revenue
//	@Accept			json
//	@Produce		json
//	@Param			start_day	query		string	false "Start day. Example 2024-12-23"
//	@Param			end_day  	query		string	false "End day. Example 2024-12-25"
//	@Success		200				{object}	PaidMPLXDailyResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/network/revenue/paid/daily [get]
func (a *statsApi) getNetworkRevenuePaidDaily(c echo.Context) (err error) {
	startDay, endDay, err := ExtractDatesFromQuery(c)
	if err != nil {
		log.Logger.StatsAPI.Errorf("ExtractDatesFromQuery: %s", err)
		return err
	}

	paidMPLX, err := a.pgStorage.GetDailyMPLXVolume(c.Request().Context(), startDay, endDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyMPLXVolume: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, getPaidMPLXDailyResponse(paidMPLX))
}

// getNetworkRevenueDistributedTotal godoc
//
//	@Summary		Get total MPLX paid to the providers
//	@Description	Returns total amount of MPLX paid to DAS providers
//	@Tags			revenue
//	@Produce		json
//	@Success		200				{object}	TotalDistributedMPLXResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/network/revenue/distributed/total [get]
func (a *statsApi) getNetworkRevenueDistributedTotal(c echo.Context) (err error) {
	totalDistributedMPLX, err := a.pgStorage.GetTotalMPLXDistributed(c.Request().Context())
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetTotalMPLXDistributed: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, TotalDistributedMPLXResponse{DistributedTotal: totalDistributedMPLX})
}

// getNetworkRevenueDistributedDaily godoc
//
//	@Summary		Get MPLX paid to the providers in a timeframe
//	@Description	Returns MPLX paid to the DAS providers in a requested timeframe
//	@Tags			revenue
//	@Accept			json
//	@Produce		json
//	@Param			start_day	query		string	false "Start day. Example 2024-12-23"
//	@Param			end_day  	query		string	false "End day. Example 2024-12-25"
//	@Success		200				{object}	DistributedMPLXDailyResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/network/revenue/distributed/daily [get]
func (a *statsApi) getNetworkRevenueDistributedDaily(c echo.Context) (err error) {
	startDay, endDay, err := ExtractDatesFromQuery(c)
	if err != nil {
		log.Logger.StatsAPI.Errorf("ExtractDatesFromQuery: %s", err)
		return err
	}

	dailyDistributedMPLX, err := a.pgStorage.GetDailyMPLXDistributed(c.Request().Context(), startDay, endDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyMPLXDistributed: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, getDistributedMPLXDailyResponse(dailyDistributedMPLX))
}

// getTotalRewardsEarned godoc
//
//	@Summary		Get total MPLX earned by providers
//	@Description	Returns total amount of MPLX earned by DAS providers
//	@Tags			revenue
//	@Produce		json
//	@Success		200				{object}	TotalRewardsEarnedResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/network/revenue/earned/total [get]
func (a *statsApi) getTotalRewardsEarned(c echo.Context) (err error) {
	totalRewardsEarned, err := a.pgStorage.GetTotalRewardsEarned(c.Request().Context())
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetTotalRewardsEarned: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, TotalRewardsEarnedResponse{TotalEarned: totalRewardsEarned})
}

// getDailyRewardsEarned godoc
//
//	@Summary		Get MPLX earned by providers in a timeframe
//	@Description	Returns MPLX earned by DAS providers in a requested timeframe
//	@Tags			revenue
//	@Accept			json
//	@Produce		json
//	@Param			start_day	query		string	false "Start day. Example 2024-12-23"
//	@Param			end_day  	query		string	false "End day. Example 2024-12-25"
//	@Success		200				{object}	DailyRewardsEarnedResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/network/revenue/earned/daily [get]
func (a *statsApi) getDailyRewardsEarned(c echo.Context) (err error) {
	startDay, endDay, err := ExtractDatesFromQuery(c)
	if err != nil {
		log.Logger.StatsAPI.Errorf("ExtractDatesFromQuery: %s", err)
		return err
	}

	dailyRewardsEarned, err := a.pgStorage.GetDailyEarnedRewards(c.Request().Context(), startDay, endDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyEarnedRewards: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, getDailyRewardsEarnedResponse(dailyRewardsEarned))
}

// getDailyRequests godoc
//
//	@Summary		Get number of requests processed daily
//	@Description	Returns number of requests processed daily, data grouped by request type
//	@Tags			metrics
//	@Accept			json
//	@Produce		json
//	@Param			start_day	query		string	false "Start day. Example 2024-12-23"
//	@Param			end_day  	query		string	false "End day. Example 2024-12-25"
//	@Success		200				{object}	DailyRequestsResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/metrics/requests/daily [get]
func (a *statsApi) getDailyRequests(c echo.Context) (err error) {
	startDay, endDay, err := ExtractDatesFromQuery(c)
	if err != nil {
		log.Logger.StatsAPI.Errorf("ExtractDatesFromQuery: %s", err)
		return err
	}

	dailyRequests, err := a.chStorage.GetDailyRequestsByChainAndType(c.Request().Context(), startDay, endDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyRequestsByChainAndType: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, getDailyRequestsResponse(dailyRequests))
}

// getDailyUsersSnapshot godoc
//
//	@Summary		Get number of gateway users
//	@Description	Returns number of gateway users
//	@Tags			metrics
//	@Accept			json
//	@Produce		json
//	@Param			start_day	query		string	false "Start day. Example 2024-12-23"
//	@Param			end_day  	query		string	false "End day. Example 2024-12-25"
//	@Success		200				{object}	DailyUsersSnapshotResponse
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Router			/metrics/users/daily [get]
func (a *statsApi) getDailyUsersSnapshot(c echo.Context) (err error) {
	startDay, endDay, err := ExtractDatesFromQuery(c)
	if err != nil {
		log.Logger.StatsAPI.Errorf("ExtractDatesFromQuery: %s", err)
		return err
	}

	dailyUsersSnapshot, err := a.pgStorage.GetDailyUserSnapshots(c.Request().Context(), startDay, endDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyUserSnapshots: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, getDailyUsersSnapshotResponse(dailyUsersSnapshot))
}
