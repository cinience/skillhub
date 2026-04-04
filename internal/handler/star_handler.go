package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/internal/middleware"
	"github.com/cinience/skillhub/internal/service"
)

type StarHandler struct {
	svc *service.SkillService
}

func NewStarHandler(svc *service.SkillService) *StarHandler {
	return &StarHandler{svc: svc}
}

// Star handles POST /api/v1/stars/:slug
func (h *StarHandler) Star(c *gin.Context) {
	user := middleware.GetUser(c)
	slug := c.Param("slug")

	if err := h.svc.Star(c.Request.Context(), user.ID, slug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "starred"})
}

// Unstar handles DELETE /api/v1/stars/:slug
func (h *StarHandler) Unstar(c *gin.Context) {
	user := middleware.GetUser(c)
	slug := c.Param("slug")

	if err := h.svc.Unstar(c.Request.Context(), user.ID, slug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "unstarred"})
}
