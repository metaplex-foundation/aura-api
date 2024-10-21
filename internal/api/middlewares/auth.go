package middlewares

import (
	"context"
	"net/http"
	"strings"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"

	"github.com/adm-metaex/aura-api/internal/pkg/log"
)

type authMiddleware struct {
	authProvider *auth.Client
	tokenCache   *cache.Cache
}

const (
	cacheTTL             = 15 * time.Minute
	cacheCleanup         = 10 * time.Second
	tokenCacheDefaultTTL = 10 * time.Minute
	bearerAuthSchema     = "bearer"
)

var (
	ErrEmptyAuthHeaders = echo.NewHTTPError(http.StatusUnauthorized, "Empty authorization headers")
	ErrTokenIsExpired   = echo.NewHTTPError(http.StatusUnauthorized, "Auth token already expired")
	ErrTokenRevoked     = echo.NewHTTPError(http.StatusUnauthorized, "Auth token already revoked")
	ErrTokenInvalid     = echo.NewHTTPError(http.StatusUnauthorized, "Auth token invalid")
	ErrUserNotFound     = echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	ErrAuthUnknown      = echo.NewHTTPError(http.StatusUnauthorized, "Auth unknown error")
)

func (a *authMiddleware) getTokenInfo(ctx context.Context, authToken string) (tokenInfo *auth.Token, err error) {
	// try to get from tokenCache
	if cachedToken, ok := a.tokenCache.Get(authToken); ok {
		if tokenInfo, ok = cachedToken.(*auth.Token); ok {
			return tokenInfo, nil
		}

		log.Logger.API.Error("authMiddleware: getTokenInfo: fail cast to *auth.Token")
	}

	tokenInfo, err = a.authProvider.VerifyIDTokenAndCheckRevoked(ctx, authToken)
	if err != nil {
		switch {
		case auth.IsIDTokenExpired(err):
			return tokenInfo, ErrTokenIsExpired
		case auth.IsIDTokenRevoked(err):
			return tokenInfo, ErrTokenRevoked
		}

		log.Logger.API.Errorf("getTokenInfo: VerifyIDTokenAndCheckRevoked: %s", err)
		return tokenInfo, ErrAuthUnknown
	}
	if tokenInfo == nil {
		return nil, ErrTokenInvalid
	}

	// use ttl not greater than tokenCacheDefaultTTL
	ttl := time.Duration(tokenInfo.Expires-time.Now().Unix()) * time.Second
	if ttl > tokenCacheDefaultTTL {
		ttl = tokenCacheDefaultTTL
	}
	a.tokenCache.Set(authToken, tokenInfo, ttl)

	return tokenInfo, nil
}

func (*authMiddleware) getAuthToken(c echo.Context) (res string, err error) {
	splittedToken := strings.SplitN(c.Request().Header.Get(echo.HeaderAuthorization), " ", 2)
	if len(splittedToken) != 2 {
		return res, ErrEmptyAuthHeaders
	}
	if !strings.EqualFold(splittedToken[0], bearerAuthSchema) || splittedToken[1] == "" {
		return res, ErrTokenInvalid
	}

	return splittedToken[1], nil
}

//func LoadUser(authProvider *auth.Client) echo.MiddlewareFunc {
//	a := authMiddleware{
//		authProvider: authProvider,
//		tokenCache:   cache.New(cacheTTL, cacheCleanup),
//	}
//
//	return func(next echo.HandlerFunc) echo.HandlerFunc {
//		return func(c echo.Context) error {
//			authToken, err := a.getAuthToken(c)
//			if err != nil {
//				return err
//			}
//			tokenInfo, err := a.getTokenInfo(c.Request().Context(), authToken)
//			if err != nil {
//				return err
//			}
//
//			user, err := a.authProvider.GetUser(c.Request().Context(), tokenInfo.UID)
//			if err != nil {
//				return ErrUserNotFound
//			}
//
//			c.(*echoUtil.CustomContext).SetFirebaseUser(user)
//
//			return next(c)
//		}
//	}
//}
