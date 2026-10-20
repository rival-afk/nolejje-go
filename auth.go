package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

var authScheme = "Bearer"

type ApiError struct {
	Code   int
	Detail string
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Detail)
}

func HttpError(code int, detail string) *ApiError {
	return &ApiError{Code: code, Detail: detail}
}

func GetUserIDFromToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(SECRET_KEY), nil
	}, jwt.WithTimeFunc(time.Now))
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, jwt.ErrTokenInvalidClaims
	}

	raw, ok := claims["user_id"]
	if !ok {
		return 0, jwt.ErrTokenInvalidClaims
	}
	switch v := raw.(type) {
	case float64:
		return int64(v), nil
	default:
		return 0, jwt.ErrTokenInvalidClaims
	}
}

const AccessCookieName = "access_token"

func setAuthCookie(c *gin.Context, token string) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(AccessCookieName, token, 1440*60, "/", "", secure, true)
}

func clearAuthCookie(c *gin.Context) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(AccessCookieName, "", -1, "/", "", secure, true)
}

func extractToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, authScheme+" ") {
		return strings.TrimSpace(header[len(authScheme)+1:])
	}
	if cookie, err := c.Cookie(AccessCookieName); err == nil && cookie != "" {
		return cookie
	}
	return ""
}

func GetUserFunc(c *gin.Context) (map[string]any, error) {
	tokenString := extractToken(c)
	if tokenString == "" {
		return nil, HttpError(http.StatusUnauthorized, "Not authenticated")
	}
	userID, err := GetUserIDFromToken(tokenString)
	if err != nil {
		var detail string
		if strings.Contains(err.Error(), "token has expired") {
			detail = "Expired Token"
		} else {
			detail = "Invalid Token"
		}
		return nil, HttpError(http.StatusUnauthorized, detail)
	}

	rows, err := Pool.Query(Ctx, "SELECT * FROM users WHERE id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, HttpError(http.StatusUnauthorized, "Invalid Token")
	}
	return users[0], nil
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := GetUserFunc(c)
		if err != nil {
			if apiErr, ok := err.(*ApiError); ok {
				c.AbortWithStatusJSON(apiErr.Code, gin.H{"detail": apiErr.Detail})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Invalid Token"})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) map[string]any {
	if user, ok := c.Get("user"); ok {
		return user.(map[string]any)
	}
	return nil
}

func ToInt64(v any) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	}
	return 0
}
