package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/internal/config"
)

type WellKnownHandler struct {
	cfg *config.Config
}

func NewWellKnownHandler(cfg *config.Config) *WellKnownHandler {
	return &WellKnownHandler{cfg: cfg}
}

// ClawHubJSON handles GET /.well-known/clawhub.json
func (h *WellKnownHandler) ClawHubJSON(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"registryUrl": h.cfg.Server.BaseURL,
		"apiVersion":  "v1",
		"endpoints": gin.H{
			"search":   "/api/v1/search",
			"skills":   "/api/v1/skills",
			"download": "/api/v1/download",
			"resolve":  "/api/v1/resolve",
			"whoami":   "/api/v1/whoami",
			"publish":  "/api/v1/skills",
		},
	})
}
