package statsapi

import (
	"net/http"

	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/labstack/echo/v4"
)

func (a *statsApi) ping(c echo.Context) (err error) {
	return c.JSON(http.StatusOK, "pong")
}

func (a *statsApi) getNetworkRevenuePaidTotal(c echo.Context) (err error) {
	totalPaidMPLX, err := a.pgStorage.GetTotalMPLXVolume(c.Request().Context())
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetTotalMPLXVolume: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, TotalPaidMPLXResponse{PaidTotal: totalPaidMPLX})
}

func (a *statsApi) getNetworkRevenuePaidDaily(c echo.Context) (err error) {
	var params StartAndEndDatesParams
	if err = c.Bind(&params); err != nil {
		return err
	}

	err = params.Validate()
	if err != nil {
		return err
	}

	paidMPLX, err := a.pgStorage.GetDailyMPLXVolume(c.Request().Context(), params.StartDay, params.EndDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyMPLXVolume: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, PaidMPLXDailyResponse{PaidDaily: paidMPLX})
}

func (a *statsApi) getNetworkRevenueDistributedTotal(c echo.Context) (err error) {
	totalDistributedMPLX, err := a.pgStorage.GetTotalMPLXDistributed(c.Request().Context())
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetTotalMPLXDistributed: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, TotalDistributedMPLXResponse{DistributedTotal: totalDistributedMPLX})
}

func (a *statsApi) getNetworkRevenueDistributedDaily(c echo.Context) (err error) {
	var params StartAndEndDatesParams
	if err = c.Bind(&params); err != nil {
		return err
	}

	err = params.Validate()
	if err != nil {
		return err
	}

	dailyDistributedMPLX, err := a.pgStorage.GetDailyMPLXDistributed(c.Request().Context(), params.StartDay, params.EndDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyMPLXDistributed: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, DistributedMPLXDailyResponse{DistributedDaily: dailyDistributedMPLX})
}

func (a *statsApi) getTotalRewardsEarned(c echo.Context) (err error) {
	totalRewardsEarned, err := a.pgStorage.GetTotalRewardsEarned(c.Request().Context())
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetTotalRewardsEarned: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, TotalRewardsEarnedResponse{TotalEarned: totalRewardsEarned})
}

func (a *statsApi) getDailyRewardsEarned(c echo.Context) (err error) {
	var params StartAndEndDatesParams
	if err = c.Bind(&params); err != nil {
		return err
	}

	err = params.Validate()
	if err != nil {
		return err
	}

	dailyRewardsEarned, err := a.pgStorage.GetDailyEarnedRewards(c.Request().Context(), params.StartDay, params.EndDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyEarnedRewards: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, DailyRewardsEarnedResponse{DailyRewardsEarned: dailyRewardsEarned})
}

func (a *statsApi) getDailyRequests(c echo.Context) (err error) {
	var params StartAndEndDatesParams
	if err = c.Bind(&params); err != nil {
		return err
	}

	err = params.Validate()
	if err != nil {
		return err
	}

	dailyRequests, err := a.chStorage.GetDailyRequestsByChainAndType(c.Request().Context(), params.StartDay, params.EndDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyRequestsByChainAndType: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, DailyRequestsResponse{DailyRequests: dailyRequests})
}

func (a *statsApi) getDailyUsersSnapshot(c echo.Context) (err error) {
	var params StartAndEndDatesParams
	if err = c.Bind(&params); err != nil {
		return err
	}

	err = params.Validate()
	if err != nil {
		return err
	}

	dailyUsersSnapshot, err := a.pgStorage.GetDailyUserSnapshots(c.Request().Context(), params.StartDay, params.EndDay)
	if err != nil {
		log.Logger.StatsAPI.Errorf("GetDailyUserSnapshots: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, DailyUsersSnapshotResponse{DailyUsersSnapshot: dailyUsersSnapshot})
}
