package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/adm-metaex/aura-api/pkg/log"
	echoUtil "github.com/adm-metaex/aura-api/pkg/util/echo"
)

// ShowUser godoc
//
//	@Summary		Get user info
//	@Description	Return object with user info
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	UserInfo
//	@Failure		400	{object}	error
//	@Failure		401	{object}	error
//	@Failure		500	{object}	error
//	@Security		ApiKeyAuth
//	@Router			/user [get]
func (a *api) getUserHandler(c echo.Context) (err error) {
	user := c.(*echoUtil.CustomContext).GetDynamicUser()
	if user == nil {
		log.Logger.API.Errorf("getUserHandler: fail to get user from context: %v", user)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// TODO
	return c.JSON(http.StatusOK, user)
}
