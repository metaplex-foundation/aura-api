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
	echoUtil "github.com/adm-metaex/aura-api/pkg/util/echo"
)

const metaplexTokenDecimals = 6

const (
	showDeletedAPIKeysParam = "show_deleted"
	tokenParam              = "token"
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
	networks := make([]string, 0, len(a.availableNetworks))
	for network := range a.availableNetworks {
		networks = append(networks, network)
	}

	return c.JSON(http.StatusOK, networks)
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
	var userModel User
	userModel.FromDBModel(&u)

	return c.JSON(http.StatusOK, userModel)
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
//	@Success		200		{string}	"Api key was deleted successfully. Return empty string"
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

func (a *api) getAPIResponseTimes(c echo.Context) (err error) {
	networks := make([]string, 0, len(a.availableNetworks))
	for network := range a.availableNetworks {
		networks = append(networks, network)
	}

	return c.JSON(http.StatusOK, networks)
}

func (a *api) getAPIRequestsVolume(c echo.Context) (err error) {
	networks := make([]string, 0, len(a.availableNetworks))
	for network := range a.availableNetworks {
		networks = append(networks, network)
	}

	return c.JSON(http.StatusOK, networks)
}

func (a *api) getAPICreditsUsage(c echo.Context) (err error) {
	networks := make([]string, 0, len(a.availableNetworks))
	for network := range a.availableNetworks {
		networks = append(networks, network)
	}

	return c.JSON(http.StatusOK, networks)
}
