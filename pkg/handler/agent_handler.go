package handler

import (
	"crypto/subtle"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/auth"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

type AgentHandler struct {
	authSvc  *auth.Service
	userRepo *repository.UserRepo
}

func NewAgentHandler(authSvc *auth.Service, userRepo *repository.UserRepo) *AgentHandler {
	return &AgentHandler{authSvc: authSvc, userRepo: userRepo}
}

// Provision handles POST /api/v1/agent/provision
// Accepts a shared secret and agent handle, auto-creates user if needed, returns API token.
func (h *AgentHandler) Provision(c *gin.Context) {
	expected := os.Getenv("SKILLHUB_AGENT_SECRET")
	if expected == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var req struct {
		Handle string `json:"handle" binding:"required"`
		Secret string `json:"secret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle and secret are required"})
		return
	}

	if len(req.Handle) < 2 || len(req.Handle) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle must be 2-64 characters"})
		return
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(req.Secret), []byte(expected)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid secret"})
		return
	}

	ctx := c.Request.Context()

	// Find or create user
	user, err := h.userRepo.GetByHandle(ctx, req.Handle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if user == nil {
		user = &model.User{
			ID:     uuid.New(),
			Handle: req.Handle,
			Role:   "agent",
		}
		if err := h.userRepo.Create(ctx, user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	if user.IsBanned {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is banned"})
		return
	}

	// Create a publish-scope token with 90-day expiry
	rawToken, _, err := h.authSvc.CreateToken(ctx, user.ID, "agent-provision", "publish", 90*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":  rawToken,
		"handle": user.Handle,
		"userId": user.ID.String(),
	})
}
