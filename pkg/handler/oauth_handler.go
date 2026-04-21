package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/auth"
)

type OAuthHandler struct {
	oauthSvc *auth.OAuthService
}

func NewOAuthHandler(oauthSvc *auth.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthSvc: oauthSvc}
}

// Redirect handles GET /auth/:provider — redirects to the OAuth provider.
func (h *OAuthHandler) Redirect(c *gin.Context) {
	provider := c.Param("provider")

	if !h.oauthSvc.HasProvider(provider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported OAuth provider"})
		return
	}

	authURL, state, err := h.oauthSvc.GetAuthURL(provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate auth URL"})
		return
	}

	// Store state in cookie (for UI round-trip) — actual validation is server-side
	secure := h.oauthSvc.IsSecure()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth_state", state, 300, "/", "", secure, true)
	c.Redirect(http.StatusFound, authURL)
}

// Callback handles GET /auth/:provider/callback — exchanges code for token.
func (h *OAuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code parameter"})
		return
	}

	// Validate state server-side
	if !h.oauthSvc.ValidateState(state) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state parameter"})
		return
	}
	// Clear state cookie
	secure := h.oauthSvc.IsSecure()
	c.SetCookie("oauth_state", "", -1, "/", "", secure, true)

	rawToken, _, err := h.oauthSvc.HandleCallback(c.Request.Context(), provider, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication failed"})
		return
	}

	// Set session cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session_token", rawToken, 30*24*3600, "/", "", secure, true)
	c.Redirect(http.StatusFound, "/")
}
