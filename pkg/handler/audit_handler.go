package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cinience/skillhub/pkg/repository"
	"github.com/cinience/skillhub/pkg/service"
)

type AuditHandler struct {
	svc *service.AuditService
}

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List handles GET /api/v1/admin/audit-logs
func (h *AuditHandler) List(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	cursor := c.Query("cursor")

	filter := repository.AuditFilter{
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
	}
	if actorStr := c.Query("actor_id"); actorStr != "" {
		if id, err := uuid.Parse(actorStr); err == nil {
			filter.ActorID = &id
		}
	}

	logs, nextCursor, err := h.svc.List(c.Request.Context(), limit, cursor, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       logs,
		"nextCursor": nextCursor,
	})
}
