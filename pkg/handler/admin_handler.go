package handler

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/middleware"
	"github.com/cinience/skillhub/pkg/model"
	"github.com/cinience/skillhub/pkg/repository"
	"github.com/cinience/skillhub/pkg/service"
)

var validRoles = []string{"user", "moderator", "admin"}

type AdminHandler struct {
	userRepo *repository.UserRepo
	skillSvc *service.SkillService
	auditSvc *service.AuditService
}

func NewAdminHandler(userRepo *repository.UserRepo, skillSvc *service.SkillService, auditSvc *service.AuditService) *AdminHandler {
	return &AdminHandler{userRepo: userRepo, skillSvc: skillSvc, auditSvc: auditSvc}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to ban user"})
		return
	}

	if h.auditSvc != nil {
		admin := middleware.GetUser(c)
		var adminID *uuid.UUID
		if admin != nil {
			adminID = &admin.ID
		}
		h.auditSvc.Log(c.Request.Context(), adminID, "ban_user", "user", &userID, req.Reason, c.ClientIP())
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		return
	}

	if h.auditSvc != nil {
		admin := middleware.GetUser(c)
		var adminID *uuid.UUID
		if admin != nil {
			adminID = &admin.ID
		}
		h.auditSvc.Log(c.Request.Context(), adminID, "set_role", "user", &userID, req.Role, c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{"message": "role updated"})
}

// ListUsers handles GET /api/v1/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	cursor := c.Query("cursor")
	users, nextCursor, err := h.userRepo.List(c.Request.Context(), 20, cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// ListAllSkills handles GET /api/v1/admin/skills
// Supports ?visibility=private|public|pending|all (default: all)
func (h *AdminHandler) ListAllSkills(c *gin.Context) {
	visibility := c.DefaultQuery("visibility", "")
	cursor := c.Query("cursor")
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	skills, nextCursor, err := h.skillSvc.ListAllSkillsForAdmin(c.Request.Context(), limit, cursor, visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list skills"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       skills,
		"nextCursor": nextCursor,
	})
}

// ReviewSkill handles POST /api/v1/admin/skills/:slug/review
func (h *AdminHandler) ReviewSkill(c *gin.Context) {
	slug := c.Param("slug")
	var req struct {
		Action string `json:"action" binding:"required"` // "approve" or "reject"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request, action is required"})
		return
	}

	approve := req.Action == "approve"
	if req.Action != "approve" && req.Action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'approve' or 'reject'"})
		return
	}

	var reviewerID *uuid.UUID
	if admin := middleware.GetUser(c); admin != nil {
		reviewerID = &admin.ID
	}

	if err := h.skillSvc.ReviewSkill(c.Request.Context(), reviewerID, slug, approve); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "skill " + req.Action + "d"})
}

// SetVisibility handles POST /api/v1/admin/skills/:slug/visibility
func (h *AdminHandler) SetVisibility(c *gin.Context) {
	slug := c.Param("slug")
	var req struct {
		Visibility string `json:"visibility" binding:"required"` // "public" or "private"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request, visibility is required"})
		return
	}

	var adminID *uuid.UUID
	if admin := middleware.GetUser(c); admin != nil {
		adminID = &admin.ID
	}

	if err := h.skillSvc.SetSkillVisibility(c.Request.Context(), adminID, slug, req.Visibility); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "visibility updated to " + req.Visibility})
}
