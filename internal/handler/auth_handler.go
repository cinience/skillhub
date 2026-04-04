package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/internal/auth"
	"github.com/cinience/skillhub/internal/middleware"
	"github.com/cinience/skillhub/internal/repository"
)

type AuthHandler struct {
	authSvc  *auth.Service
	userRepo *repository.UserRepo
}

func NewAuthHandler(authSvc *auth.Service, userRepo *repository.UserRepo) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, userRepo: userRepo}
}

// WhoAmI handles GET /api/v1/whoami
func (h *AuthHandler) WhoAmI(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// CreateToken handles POST /api/v1/tokens
func (h *AuthHandler) CreateToken(c *gin.Context) {
	var req struct {
		UserID string `json:"userId"`
		Label  string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	rawToken, token, err := h.authSvc.CreateToken(c.Request.Context(), userID, req.Label)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":    rawToken,
		"metadata": token,
	})
}
