package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/middleware"
	"github.com/cinience/skillhub/pkg/service"
)

type DownloadHandler struct {
	svc *service.SkillService
}

func NewDownloadHandler(svc *service.SkillService) *DownloadHandler {
	return &DownloadHandler{svc: svc}
}

// Download handles GET /api/v1/download
func (h *DownloadHandler) Download(c *gin.Context) {
	slug := c.Query("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	version := c.DefaultQuery("version", "latest")

	// Create identity hash from IP for dedup
	hash := sha256.Sum256([]byte(c.ClientIP()))
	identityHash := hex.EncodeToString(hash[:])

	viewer := middleware.GetUser(c)
	archive, filename, err := h.svc.Download(c.Request.Context(), slug, version, identityHash, viewer)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer archive.Close()

	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("Content-Type", "application/zip")

	if _, err := io.Copy(c.Writer, archive); err != nil {
		log.Printf("download stream error for %s: %v", slug, err)
	}
}

// Resolve handles GET /api/v1/resolve
func (h *DownloadHandler) Resolve(c *gin.Context) {
	fingerprint := c.Query("fingerprint")
	if fingerprint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fingerprint is required"})
		return
	}

	version, skill, err := h.svc.ResolveFingerprint(c.Request.Context(), fingerprint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if version == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"version": version,
		"skill":   skill,
	})
}
