package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/auth"
	"github.com/cinience/skillhub/pkg/model"
)

const (
	UserContextKey = "user"
)

func RequireAuth(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := extractUser(c, authSvc)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}
		c.Set(UserContextKey, user)
		c.Next()
	}
}

func OptionalAuth(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := extractUser(c, authSvc)
		if user != nil {
			c.Set(UserContextKey, user)
		}
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, exists := c.Get(UserContextKey)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}
		user := u.(*model.User)
		for _, role := range roles {
			if user.Role == role {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

func extractUser(c *gin.Context, authSvc *auth.Service) *model.User {
	// Try Authorization header first
	header := c.GetHeader("Authorization")
	if header != "" {
		token := strings.TrimPrefix(header, "Bearer ")
		if token != header {
			user, err := authSvc.ValidateToken(c.Request.Context(), token)
			if err == nil && user != nil {
				return user
			}
		}
	}

	// Fall back to session cookie
	if token, err := c.Cookie("session_token"); err == nil && token != "" {
		user, err := authSvc.ValidateToken(c.Request.Context(), token)
		if err == nil && user != nil {
			return user
		}
	}

	return nil
}

// WebOptionalAuth is an alias for OptionalAuth, used on web routes.
// It reads from both Authorization header and session_token cookie.
func WebOptionalAuth(authSvc *auth.Service) gin.HandlerFunc {
	return OptionalAuth(authSvc)
}

// GetUser retrieves the authenticated user from the context.
func GetUser(c *gin.Context) *model.User {
	u, exists := c.Get(UserContextKey)
	if !exists {
		return nil
	}
	return u.(*model.User)
}
