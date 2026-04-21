package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/auth"
	"github.com/cinience/skillhub/pkg/model"
)

const (
	UserContextKey      = "user"
	TokenScopeKey       = "token_scope"
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
			user, scope, err := authSvc.ValidateToken(c.Request.Context(), token)
			if err == nil && user != nil {
				c.Set(TokenScopeKey, scope)
				return user
			}
		}
	}

	// Fall back to session cookie
	if token, err := c.Cookie("session_token"); err == nil && token != "" {
		user, scope, err := authSvc.ValidateToken(c.Request.Context(), token)
		if err == nil && user != nil {
			c.Set(TokenScopeKey, scope)
			return user
		}
	}

	return nil
}

// RequireScope checks that the token scope allows the current request method.
// Scope rules: "read" → GET only; "publish" → GET + POST /skills; "full" → all.
func RequireScope(allowedScopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeVal, exists := c.Get(TokenScopeKey)
		if !exists {
			// No scope set means session cookie (full access)
			c.Next()
			return
		}
		scope := scopeVal.(string)
		if scope == "full" {
			c.Next()
			return
		}

		method := c.Request.Method

		switch scope {
		case "read":
			if method != "GET" && method != "HEAD" {
				c.JSON(http.StatusForbidden, gin.H{"error": "token scope 'read' only allows GET requests"})
				c.Abort()
				return
			}
		case "publish":
			if method != "GET" && method != "HEAD" {
				if method != "POST" {
					c.JSON(http.StatusForbidden, gin.H{"error": "token scope 'publish' does not allow this method"})
					c.Abort()
					return
				}
				// Restrict POST to skill publish/star routes only
				path := c.FullPath()
				if path != "/api/v1/skills" && path != "/api/v1/stars/:slug" {
					c.JSON(http.StatusForbidden, gin.H{"error": "token scope 'publish' only allows publishing skills"})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// GetUser retrieves the authenticated user from the context.
func GetUser(c *gin.Context) *model.User {
	u, exists := c.Get(UserContextKey)
	if !exists {
		return nil
	}
	return u.(*model.User)
}
