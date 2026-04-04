package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/auth"
)

// WebAuthHandler handles login/logout for the SPA frontend.
// These remain server-side to manage httpOnly session cookies.
type WebAuthHandler struct {
	authSvc *auth.Service
}

func NewWebAuthHandler(authSvc *auth.Service) *WebAuthHandler {
	return &WebAuthHandler{authSvc: authSvc}
}

// LoginSubmit handles POST /login — validates credentials and sets session cookie.
func (h *WebAuthHandler) LoginSubmit(c *gin.Context) {
	handle := strings.TrimSpace(c.PostForm("handle"))
	password := c.PostForm("password")

	if handle == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	rawToken, _, err := h.authSvc.Login(c.Request.Context(), handle, password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// Set session cookie (httpOnly, SameSite=Lax, Secure when behind TLS)
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session_token", rawToken, 30*24*3600, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Logout clears the session cookie.
func (h *WebAuthHandler) Logout(c *gin.Context) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetCookie("session_token", "", -1, "/", "", secure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
