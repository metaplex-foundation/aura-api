package middlewares

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"

	"github.com/adm-metaex/aura-api/pkg/dynamic"
	"github.com/adm-metaex/aura-api/pkg/log"
	echoUtil "github.com/adm-metaex/aura-api/pkg/util/echo"
)

type AuthMiddleware struct {
	dynamicClient *dynamic.Client
	jwksClient    *keyfunc.JWKS
	tokenCache    *cache.Cache
}

const (
	cacheTTL             = 15 * time.Minute
	cacheCleanup         = 10 * time.Second
	tokenCacheDefaultTTL = 10 * time.Minute
	bearerAuthSchema     = "bearer"
)

var (
	ErrEmptyAuthHeaders            = echo.NewHTTPError(http.StatusUnauthorized, "Empty authorization headers")
	ErrTokenIsExpired              = echo.NewHTTPError(http.StatusUnauthorized, "Auth token already expired")
	ErrTokenRequiresAdditionalAuth = echo.NewHTTPError(http.StatusUnauthorized, "JWT requires additional auth")
	ErrTokenInvalid                = echo.NewHTTPError(http.StatusUnauthorized, "Auth token invalid")
	ErrTokenWithoutExpire          = echo.NewHTTPError(http.StatusUnauthorized, "Auth token do not expiration time")
	ErrUserNotFound                = echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	ErrAuthUnknown                 = echo.NewHTTPError(http.StatusUnauthorized, "Auth unknown error")
)

func NewAuthMiddleware(ctx context.Context, jwksEndpointURL string, dynamicClient *dynamic.Client) (a *AuthMiddleware, err error) {
	options := keyfunc.Options{
		RefreshErrorHandler: func(err error) {
			log.Logger.API.Errorf("JWKS refresh: %s", err)
		},
		RefreshInterval:   time.Hour, // Refresh JWKS interval
		RefreshUnknownKID: true,
		RefreshTimeout:    10 * time.Second,
		Ctx:               ctx,
	}

	jwks, err := keyfunc.Get(jwksEndpointURL, options)
	if err != nil {
		return nil, fmt.Errorf("keyfunc.Get: %w", err)
	}

	a = &AuthMiddleware{
		jwksClient:    jwks,
		tokenCache:    cache.New(cacheTTL, cacheCleanup),
		dynamicClient: dynamicClient,
	}

	return a, nil
}

func isTokenExpired(err error) bool {
	var jwtError *jwt.ValidationError
	if errors.As(err, &jwtError) {
		if jwtError.Errors == jwt.ValidationErrorExpired {
			return true
		}
	}

	return false
}

func (a *AuthMiddleware) getTokenInfo(authToken string) (userID string, err error) {
	// try to get from tokenCache
	if cachedToken, ok := a.tokenCache.Get(authToken); ok {
		if userID, ok = cachedToken.(string); ok {
			return userID, nil
		}

		log.Logger.API.Error("authMiddleware: getTokenInfo: fail cast to *jwt.Token")
	}

	tokenInfo, err := jwt.Parse(authToken, a.jwksClient.Keyfunc)
	if err != nil {
		if isTokenExpired(err) {
			return userID, ErrTokenIsExpired
		}

		log.Logger.API.Errorf("getTokenInfo: jwt.Parse: %s", err)
		return userID, ErrAuthUnknown
	}
	if tokenInfo == nil || !tokenInfo.Valid {
		return userID, ErrTokenInvalid
	}
	claims, ok := tokenInfo.Claims.(jwt.MapClaims)
	if !ok {
		log.Logger.API.Error("authMiddleware: getTokenInfo: fail cast to jwt.MapClaims")
		return userID, ErrTokenInvalid
	}
	if scopes, ok := claims["scopes"].([]interface{}); ok {
		for _, scope := range scopes {
			if scopeStr, ok := scope.(string); ok && scopeStr == "requiresAdditionalAuth" {
				return userID, ErrTokenRequiresAdditionalAuth
			}
		}
	}
	expiredAt, ok := claims["exp"]
	if !ok {
		return userID, ErrTokenWithoutExpire
	}
	var expireTimestamp float64
	switch exp := expiredAt.(type) {
	case float64:
		expireTimestamp = exp
	case json.Number:
		expireTimestamp, err = exp.Float64()
		if err != nil {
			log.Logger.API.Errorf("authMiddleware: getTokenInfo: exp.Float64: %s", err)
			return userID, ErrTokenWithoutExpire
		}
	default:
		return userID, ErrTokenWithoutExpire
	}
	usrID, ok := claims["sub"]
	if !ok {
		log.Logger.API.Error("authMiddleware: getTokenInfo: no userID in claims")
		return userID, ErrUserNotFound
	}
	userID, ok = usrID.(string)
	if !ok {
		log.Logger.API.Error("authMiddleware: getTokenInfo: no userIDString in claims")
		return userID, ErrUserNotFound
	}

	// use ttl not greater than tokenCacheDefaultTTL
	ttl := time.Duration(int64(expireTimestamp)-time.Now().Unix()) * time.Second
	if ttl > tokenCacheDefaultTTL {
		ttl = tokenCacheDefaultTTL
	}
	a.tokenCache.Set(authToken, userID, ttl)

	return userID, nil
}

func (*AuthMiddleware) getAuthToken(c echo.Context) (res string, err error) {
	splittedToken := strings.SplitN(c.Request().Header.Get(echo.HeaderAuthorization), " ", 2)
	if len(splittedToken) != 2 {
		return res, ErrEmptyAuthHeaders
	}
	if !strings.EqualFold(splittedToken[0], bearerAuthSchema) || splittedToken[1] == "" {
		return res, ErrTokenInvalid
	}

	return splittedToken[1], nil
}

func (a *AuthMiddleware) LoadUser() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authToken, err := a.getAuthToken(c)
			if err != nil {
				return err
			}
			userID, err := a.getTokenInfo(authToken)
			if err != nil {
				return err
			}

			user, err := a.dynamicClient.GetUserByID(c.Request().Context(), userID)
			if err != nil {
				return ErrUserNotFound
			}

			c.(*echoUtil.CustomContext).SetDynamicUser(user)

			return next(c)
		}
	}
}
