package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"

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
	viewer := middleware.GetUser(c)

	// Fast path: if client sent If-None-Match and fingerprint matches current version,
	// return 304 without recording a download or building the zip.
	if inm := c.GetHeader("If-None-Match"); inm != "" {
		_, ver, err := h.svc.ResolveVersion(c.Request.Context(), slug, version, viewer)
		if err == nil && ver != nil && matchesETag(inm, ver.Fingerprint) {
			c.Header("ETag", quoteETag(ver.Fingerprint))
			c.Status(http.StatusNotModified)
			return
		}
	}

	// Create identity hash from IP for dedup
	hash := sha256.Sum256([]byte(c.ClientIP()))
	identityHash := hex.EncodeToString(hash[:])

	result, err := h.svc.Download(c.Request.Context(), slug, version, identityHash, viewer)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer result.Archive.Close()

	c.Header("ETag", quoteETag(result.Fingerprint))
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": result.Filename}))
	c.Header("Content-Type", "application/zip")
	c.Header("X-Skill-Version", result.Version)
	c.Header("X-Skill-Fingerprint", result.Fingerprint)

	if _, err := io.Copy(c.Writer, result.Archive); err != nil {
		log.Printf("download stream error for %s: %v", slug, err)
	}
}

// quoteETag wraps a fingerprint in quotes per RFC 7232.
func quoteETag(fp string) string { return "\"" + fp + "\"" }

// matchesETag reports whether an If-None-Match header value covers the given fingerprint.
// Handles comma-separated tags, optional weak prefix, and wildcard "*".
func matchesETag(header, fingerprint string) bool {
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if tag == "*" {
			return true
		}
		tag = strings.TrimPrefix(tag, "W/")
		tag = strings.Trim(tag, "\"")
		if tag == fingerprint {
			return true
		}
	}
	return false
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
