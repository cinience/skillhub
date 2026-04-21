package handler

import (
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/auth"
	"github.com/cinience/skillhub/pkg/middleware"
	"github.com/cinience/skillhub/pkg/repository"
)

var validScopes = []string{"full", "read", "publish"}

type TokenHandler struct {
	authSvc   *auth.Service
	tokenRepo *repository.TokenRepo
}

func NewTokenHandler(authSvc *auth.Service, tokenRepo *repository.TokenRepo) *TokenHandler {
	return &TokenHandler{authSvc: authSvc, tokenRepo: tokenRepo}
}

// List handles GET /api/v1/tokens — list current user's tokens.
func (h *TokenHandler) List(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	tokens, err := h.tokenRepo.GetByUserID(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tokens})
}

// Create handles POST /api/v1/tokens — create a new token for the current user.
func (h *TokenHandler) Create(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		Label     string `json:"label"`
		Scope     string `json:"scope"`
		ExpiresIn string `json:"expiresIn"` // e.g. "720h" for 30 days
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	scope := req.Scope
	if scope == "" {
		scope = "full"
	}
	if !slices.Contains(validScopes, scope) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope, must be one of: full, read, publish"})
		return
	}

	var expiresIn time.Duration
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil || d <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expiresIn duration"})
			return
		}
		expiresIn = d
	}

	rawToken, token, err := h.authSvc.CreateToken(c.Request.Context(), user.ID, req.Label, scope, expiresIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":    rawToken,
		"metadata": token,
	})
}

// Revoke handles DELETE /api/v1/tokens/:id — revoke a token owned by the current user.
func (h *TokenHandler) Revoke(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token ID"})
		return
	}

	token, err := h.tokenRepo.GetByID(c.Request.Context(), tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if token == nil || token.UserID != user.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	if err := h.tokenRepo.Revoke(c.Request.Context(), tokenID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "token revoked"})
}
