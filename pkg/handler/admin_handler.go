package handler

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
)

var validRoles = []string{"user", "moderator", "admin"}

type AdminHandler struct {
	userRepo *repository.UserRepo
}

func NewAdminHandler(userRepo *repository.UserRepo) *AdminHandler {
	return &AdminHandler{userRepo: userRepo}
}

// BanUser handles POST /api/v1/users/ban
func (h *AdminHandler) BanUser(c *gin.Context) {
	var req struct {
		UserID string `json:"userId"`
		Reason string `json:"reason"`
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

	if err := h.userRepo.Ban(c.Request.Context(), userID, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

// SetRole handles POST /api/v1/users/role
func (h *AdminHandler) SetRole(c *gin.Context) {
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if !slices.Contains(validRoles, req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be one of: user, moderator, admin"})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.userRepo.UpdateRole(c.Request.Context(), userID, req.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

// ListUsers handles GET /api/v1/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	cursor := c.Query("cursor")
	users, nextCursor, err := h.userRepo.List(c.Request.Context(), 20, cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       users,
		"nextCursor": nextCursor,
	})
}

// CreateUser handles POST /api/v1/users (admin)
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req struct {
		Handle      string `json:"handle" binding:"required"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Role        string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	role := "user"
	if req.Role != "" {
		if !slices.Contains(validRoles, req.Role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be one of: user, moderator, admin"})
			return
		}
		role = req.Role
	}

	user := &model.User{
		ID:     uuid.New(),
		Handle: req.Handle,
		Role:   role,
	}
	if req.DisplayName != "" {
		user.DisplayName = &req.DisplayName
	}
	if req.Email != "" {
		user.Email = &req.Email
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}
