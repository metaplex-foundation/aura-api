package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/log"
	"github.com/adm-metaex/aura-api/pkg/util"
	echoUtil "github.com/adm-metaex/aura-api/pkg/util/echo"
)

const (
	basicAPIKeyName = "Basic API key"
)

const (
	showDeletedAPIKeysParam = "show_deleted"
	tokenParam              = "token"
	networkParam            = "network"
	methodParam             = "method"
	timeframeParam          = "timeframe"
	granularityParam        = "granularity"
)

var (
	ErrAPIKeyNotFound          = "API key not found"
	ErrApiKeyNameAlreadyExists = "API key name already exists"
	ErrUserNotFound            = "User not found"
)

// getSupportedNetworksHandler godoc
//
//	@Summary		Get list of supported networks
//	@Description	Return list of supported networks
//	@Tags			networks
//	@Produce		json
//	@Success		200	{array}		string
//	@Failure		400	{object}	error
//	@Failure		401	{object}	error
//	@Failure		500	{object}	error
//	@Router			/networks [get]
func (a *api) getSupportedNetworksHandler(c echo.Context) (err error) {
	return c.JSON(http.StatusOK, util.MapKeys(a.availableNetworks))
}

// getUserHandler godoc
//
//	@Summary		Get user info
//	@Description	Return object with user info
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	User
//	@Failure		400	{object}	error
//	@Failure		401	{object}	error
//	@Failure		500	{object}	error
//	@Security		ApiKeyAuth
//	@Router			/user [get]
func (a *api) getUserHandler(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("getUserHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	u, err := a.pgStorage.GetOrCreateUser(c.Request().Context(), user.ID)
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, ErrUserNotFound)
		}
		log.Logger.API.Errorf("getUserHandler: GetOrCreateUser: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, a.UserWithCurrentPlanFromDBModel(&u))
}

// createAPIKeyHandler godoc
//
//	@Summary		Create api key
//	@Description	Create api key
//	@Tags			api key
//	@Accept			json
//	@Produce		json
//	@Param			request_body	body		CreateAPIKeyRequestParams	true	"API key creation request"
//	@Success		201				{object}	postgres.APIKeyWithSupportedNetworks
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Security		ApiKeyAuth
//	@Router			/keys [post]
func (a *api) createAPIKeyHandler(c echo.Context) (err error) {
	var params CreateAPIKeyRequestParams
	if err = c.Bind(&params); err != nil {
		return err
	}
	if err = params.Validate(a.availableNetworks); err != nil {
		return err
	}

	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("createAPIKeyHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	u, err := a.pgStorage.GetOrCreateUser(c.Request().Context(), user.ID)
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, ErrUserNotFound)
		}
		log.Logger.API.Errorf("createAPIKeyHandler: GetOrCreateUser: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	networkIDs := make([]int64, 0, len(params.Networks))
	for _, network := range params.Networks {
		nID, ok := a.availableNetworks[network]
		if !ok {
			log.Logger.API.Errorf("createAPIKeyHandler: invalid network: %v", network)
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Selected invalid network: %s", network))
		}
		networkIDs = append(networkIDs, nID)
	}
	apiKey, err := a.pgStorage.CreateAPIKey(c.Request().Context(), u.ID, params.Name, networkIDs)
	if err != nil {
		if postgres.IsErrAPIKeysLimitReached(err) {
			return echo.NewHTTPError(http.StatusBadRequest, postgres.APIKeysLimitReachedErrorText)
		}
		if postgres.IsErrViolateConstraint(err) {
			return echo.NewHTTPError(http.StatusBadRequest, ErrApiKeyNameAlreadyExists)
		}
		log.Logger.API.Errorf("createAPIKeyHandler: CreateAPIKey: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	lastUsed := time.Now()
	apiKey.LastUsed = &lastUsed
	apiKey.TotalRequests = 1000

	return c.JSON(http.StatusCreated, apiKey)
}

// apiKeyHandler godoc
//
//	@Summary		Get api key by token
//	@Description	Get api key by token
//	@Tags			api key
//	@Accept			json
//	@Produce		json
//	@Param			token	path		string	true	"Token parameter"	Format(uuid)	example(98379b6b-dc6a-4d8e-8271-12eed4822afc)
//	@Success		200		{object}	postgres.APIKeyWithSupportedNetworks
//	@Failure		400		{object}	error
//	@Failure		401		{object}	error
//	@Failure		500		{object}	error
//	@Security		ApiKeyAuth
//	@Router			/keys/{token} [get]
func (a *api) apiKeyHandler(c echo.Context) (err error) { //nolint:dupl
	apiKeyToken, err := uuid.Parse(c.Param(tokenParam))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid uuid for token parameter: %s", c.Param(tokenParam)))
	}

	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("apiKeyHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	apiKey, err := a.pgStorage.GetAPIKeyByTokenAndUserDynamicID(c.Request().Context(), apiKeyToken)
	if errors.Is(err, pg.ErrNoRows) {
		return echo.NewHTTPError(http.StatusBadRequest, ErrAPIKeyNotFound)
	} else if err != nil {
		log.Logger.API.Errorf("apiKeyHandler: GetAPIKeyByTokenAndUserDynamicID: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	lastUsed := time.Now()
	apiKey.LastUsed = &lastUsed
	apiKey.TotalRequests = 1000

	return c.JSON(http.StatusOK, apiKey)
}

// apiKeysHandler godoc
//
//	@Summary		Get api keys for user
//	@Description	Get api keys for user
//	@Tags			api key
//	@Accept			json
//	@Produce		json
//	@Param			show_deleted	query		bool	false	"Define if we need to show deleted keys. If the parameter is not present - show only living keys. If show_deleted == true only deleted keys will be returned. If show_deleted == false all keys (living and deleted) will be returned"
//	@Success		200				{array}		postgres.APIKeyWithSupportedNetworks
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Security		ApiKeyAuth
//	@Router			/keys [get]
func (a *api) apiKeysHandler(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("apiKeysHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	u, err := a.pgStorage.GetOrCreateUser(c.Request().Context(), user.ID)
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, ErrUserNotFound)
		}
		log.Logger.API.Errorf("apiKeysHandler: GetOrCreateUser: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	var showDeleted *bool
	if showDeletedAPIKeysString := c.QueryParam(showDeletedAPIKeysParam); showDeletedAPIKeysString != "" {
		v, err := strconv.ParseBool(showDeletedAPIKeysString)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, showDeletedAPIKeysParam)
		}
		showDeleted = &v
	}

	apiKeys, err := a.pgStorage.GetAPIKeysByUser(c.Request().Context(), u.ID, showDeleted)
	if err != nil {
		log.Logger.API.Errorf("apiKeysHandler: GetAPIKeysByUser: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	if len(apiKeys) == 0 {
		apiKey, err := a.pgStorage.CreateAPIKey(c.Request().Context(), u.ID, basicAPIKeyName, util.MapValues(a.availableNetworks))
		if err != nil {
			log.Logger.API.Errorf("apiKeysHandler: CreateAPIKey: %s", err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		apiKeys = append(apiKeys, apiKey)
	}
	// TODO: remove
	for i := range apiKeys {
		lastUsed := time.Now()
		apiKeys[i].LastUsed = &lastUsed
		apiKeys[i].TotalRequests = 1000
	}

	return c.JSON(http.StatusOK, apiKeys)
}

// updateAPIKeyHandler godoc
//
//	@Summary		Update api key
//	@Description	Update api key
//	@Tags			api key
//	@Accept			json
//	@Produce		json
//	@Param			token			path		string						true	"Token parameter"	Format(uuid)	example(98379b6b-dc6a-4d8e-8271-12eed4822afc)
//	@Param			request_body	body		UpdateAPIKeyRequestParams	true	"API key update request"
//	@Success		200				{object}	postgres.APIKeyWithSupportedNetworks
//	@Failure		400				{object}	error
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Security		ApiKeyAuth
//	@Router			/keys/{token} [patch]
func (a *api) updateAPIKeyHandler(c echo.Context) (err error) {
	var params UpdateAPIKeyRequestParams
	if err = c.Bind(&params); err != nil {
		return err
	}
	if err = params.Validate(a.availableNetworks); err != nil {
		return err
	}

	apiKeyToken, err := uuid.Parse(c.Param(tokenParam))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid uuid for token parameter: %s", c.Param(tokenParam)))
	}

	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("updateAPIKeyHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	networkIDs := make([]int64, 0, len(params.Networks))
	for _, network := range params.Networks {
		nID, ok := a.availableNetworks[network]
		if !ok {
			log.Logger.API.Errorf("updateAPIKeyHandler: invalid network: %v", network)
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Selected invalid network: %s", network))
		}
		networkIDs = append(networkIDs, nID)
	}
	apiKey, err := a.pgStorage.UpdateAPIKey(c.Request().Context(), apiKeyToken, params.Name, networkIDs)
	if errors.Is(err, pg.ErrNoRows) {
		return echo.NewHTTPError(http.StatusBadRequest, ErrAPIKeyNotFound)
	}
	if postgres.IsErrViolateConstraint(err) {
		return echo.NewHTTPError(http.StatusBadRequest, ErrApiKeyNameAlreadyExists)
	}
	if err != nil {
		log.Logger.API.Errorf("updateAPIKeyHandler: UpdateAPIKey: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	lastUsed := time.Now()
	apiKey.LastUsed = &lastUsed
	apiKey.TotalRequests = 1000

	return c.JSON(http.StatusOK, apiKey)
}

// deleteAPIKeyHandler godoc
//
//	@Summary		Delete api key
//	@Description	Delete api key
//	@Tags			api key
//	@Accept			json
//	@Produce		json
//	@Param			token	path		string	true	"Token parameter"	Format(uuid)	example(98379b6b-dc6a-4d8e-8271-12eed4822afc)
//	@Success		200		{string}	string	"Api key was deleted successfully. Return empty string"
//	@Failure		400		{object}	error
//	@Failure		401		{object}	error
//	@Failure		500		{object}	error
//	@Security		ApiKeyAuth
//	@Router			/keys/{token} [delete]
func (a *api) deleteAPIKeyHandler(c echo.Context) (err error) {
	apiKeyToken, err := uuid.Parse(c.Param(tokenParam))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid uuid for token parameter: %s", c.Param(tokenParam)))
	}

	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("deleteAPIKeyHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	err = a.pgStorage.DeleteAPIKey(c.Request().Context(), apiKeyToken, user.ID)
	if errors.Is(err, pg.ErrNoRows) {
		return echo.NewHTTPError(http.StatusBadRequest, ErrAPIKeyNotFound)
	}
	if err != nil {
		log.Logger.API.Errorf("deleteAPIKeyHandler: DeleteAPIKey: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

// getAPIResponseTimes godoc
//
//	@Summary		Get response time history (avg && p95) for user requests
//	@Description	Get response time history (avg && p95) for user requests
//	@Tags			stats
//	@Produce		json
//	@Param			granularity	query		string	true	"Request granularity (1 candle size). Can be either 1d (1 day) or 1h (1 hour)"
//	@Param			timeframe	query		string	true	"Request timeframe. Can be one of the following: [1h, 4h, 12h, 1d, 7d, 14d, 30d]"
//	@Param			token		query		string	false	"User api token"	Format(uuid)	example(98379b6b-dc6a-4d8e-8271-12eed4822afc)
//	@Param			network		query		string	false	"Network where requests were executed"
//	@Param			method		query		string	false	"RPC method. If indicated, require paas network parameter too"
//	@Success		200			{array}		clickhouse.ResponseTimeHistory
//	@Failure		400			{object}	error
//	@Failure		401			{object}	error
//	@Failure		500			{object}	error
//	@Security		ApiKeyAuth
//	@Router			/stats/response/time [get]
func (a *api) getAPIResponseTimes(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("getAPIResponseTimes: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	var params StatsRequestParams
	err = params.Bind(c, a.availableNetworks)
	if err != nil {
		return err
	}

	responseTimeHistory, err := a.chStorage.GetResponseTimeHistory(user.ID, params.TokenUUID, params.Network, params.RPCMethod, params.StartTime, params.Granularity)
	if err != nil {
		log.Logger.API.Errorf("GetResponseTimeHistory: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, responseTimeHistory)
}

// getAPIRequestsVolume godoc
//
//	@Summary		Get user requests volume
//	@Description	Get user requests volume
//	@Tags			stats
//	@Produce		json
//	@Param			granularity	query		string	true	"Request granularity (1 candle size). Can be either 1d (1 day) or 1h (1 hour)"
//	@Param			timeframe	query		string	true	"Request timeframe. Can be one of the following: [1h, 4h, 12h, 1d, 7d, 14d, 30d]"
//	@Param			token		query		string	false	"User api token"	Format(uuid)	example(98379b6b-dc6a-4d8e-8271-12eed4822afc)
//	@Param			network		query		string	false	"Network where requests were executed"
//	@Param			method		query		string	false	"RPC method. If indicated, require paas network parameter too"
//	@Success		200			{array}		clickhouse.RequestsVolumeHistory
//	@Failure		400			{object}	error
//	@Failure		401			{object}	error
//	@Failure		500			{object}	error
//	@Security		ApiKeyAuth
//	@Router			/stats/request/volume [get]
func (a *api) getAPIRequestsVolume(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("getAPIRequestsVolume: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	var params StatsRequestParams
	err = params.Bind(c, a.availableNetworks)
	if err != nil {
		return err
	}
	requestsVolumeHistory, err := a.chStorage.GetRequestsVolumeHistory(user.ID, params.TokenUUID, params.Network, params.RPCMethod, params.StartTime, params.Granularity)
	if err != nil {
		log.Logger.API.Errorf("GetRequestsVolumeHistory: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, requestsVolumeHistory)
}

// getAPICreditsUsage godoc
//
//	@Summary		Get user credits usage
//	@Description	Get user credits usage
//	@Tags			stats
//	@Produce		json
//	@Param			granularity	query		string	true	"Request granularity (1 candle size). Can be either 1d (1 day) or 1h (1 hour)"
//	@Param			timeframe	query		string	true	"Request timeframe. Can be one of the following: [1h, 4h, 12h, 1d, 7d, 14d, 30d]"
//	@Param			token		query		string	false	"User api token"	Format(uuid)	example(98379b6b-dc6a-4d8e-8271-12eed4822afc)
//	@Param			network		query		string	false	"Network where requests were executed"
//	@Param			method		query		string	false	"RPC method. If indicated, require paas network parameter too"
//	@Success		200			{array}		clickhouse.CreditsUsageHistory
//	@Failure		400			{object}	error
//	@Failure		401			{object}	error
//	@Failure		500			{object}	error
//	@Security		ApiKeyAuth
//	@Router			/stats/credits/usage [get]
func (a *api) getAPICreditsUsage(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("getAPICreditsUsage: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	var params StatsRequestParams
	err = params.Bind(c, a.availableNetworks)
	if err != nil {
		return err
	}
	creditsUsageHistory, err := a.chStorage.GetCreditsUsageHistory(user.ID, params.TokenUUID, params.Network, params.RPCMethod, params.StartTime, params.Granularity)
	if err != nil {
		log.Logger.API.Errorf("GetCreditsUsageHistory: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, creditsUsageHistory)
}

// getSubscriptionPlans godoc
//
//	@Summary		Get subscriptions plan info
//	@Description	Get subscriptions plan info
//	@Tags			networks
//	@Produce		json
//	@Success		200	{array}		SubscriptionWithPricing "If there is no monthly_price_mplx in response - it is Pay As You Go plan and we need to use price_mplx inside each pricing. If monthly_price_mplx present - we need to use it"
//	@Failure		400	{object}	error
//	@Failure		401	{object}	error
//	@Failure		500	{object}	error
//	@Router			/plans [get]
func (a *api) getSubscriptionPlans(c echo.Context) (err error) {
	subscriptionsList, err := a.pgStorage.GetSubscriptionsList(c.Request().Context())
	if err != nil {
		log.Logger.API.Errorf("getSubscriptionPlans: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, a.getSubscriptionsWithPricingList(subscriptionsList))
}

// getSubscriptionPlans godoc
//
//	@Summary		Get subscriptions plan info
//	@Description	Get subscriptions plan info
//	@Tags			users
//	@Produce		json
//	@Param			request_body	body		UpdateSubscriptionParams	true	"Subscription ID to change plan"
//	@Success		200				{string}	string						"Subscription was changed successfully. Return empty string"
//	@Failure		400				{object}	error						"Subscription changes are allowed only once every 24 hours."
//	@Failure		400				{object}	error						"Insufficient balance to change subscription."
//	@Failure		400				{object}	error						"Cannot switch to the selected subscription."
//	@Failure		401				{object}	error
//	@Failure		500				{object}	error
//	@Security		ApiKeyAuth
//	@Router			/plan [patch]
func (a *api) updateSubscriptionPlan(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil || user.ID == "" {
		log.Logger.API.Errorf("updateSubscriptionPlan: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	var params UpdateSubscriptionParams
	if err = c.Bind(&params); err != nil {
		return err
	}
	u, err := a.pgStorage.GetOrCreateUser(c.Request().Context(), user.ID)
	if err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, ErrUserNotFound)
		}
		log.Logger.API.Errorf("updateSubscriptionPlan: GetOrCreateUser: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	err = a.pgStorage.UpdateUserSubscriptionPlan(c.Request().Context(), u.ID, params.SubscriptionID)
	if updateSubscriptionErrorMessage := postgres.UpdateSubscriptionErrorMessage(err); updateSubscriptionErrorMessage != nil {
		return echo.NewHTTPError(http.StatusBadRequest, updateSubscriptionErrorMessage)
	}
	if postgres.IsErrInvalidSubscriptionID(err) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid subscription ID: %d", params.SubscriptionID))
	}
	if err != nil {
		log.Logger.API.Errorf("updateSubscriptionPlan: UpdateUserSubscriptionPlan: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
