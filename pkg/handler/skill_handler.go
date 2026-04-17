package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/middleware"
	"github.com/cinience/skillhub/pkg/service"
)

type SkillHandler struct {
	svc *service.SkillService
}

func NewSkillHandler(svc *service.SkillService) *SkillHandler {
	return &SkillHandler{svc: svc}
}

// List handles GET /api/v1/skills
func (h *SkillHandler) List(c *gin.Context) {
	cursor := c.Query("cursor")
	sort := c.DefaultQuery("sort", "created")
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	skills, nextCursor, err := h.svc.ListSkills(c.Request.Context(), limit, cursor, sort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       skills,
		"nextCursor": nextCursor,
	})
}

// Get handles GET /api/v1/skills/:slug
func (h *SkillHandler) Get(c *gin.Context) {
	slug := c.Param("slug")
	skill, err := h.svc.GetSkill(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if skill == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	c.JSON(http.StatusOK, skill)
}

// Publish handles POST /api/v1/skills
func (h *SkillHandler) Publish(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	if err := c.Request.ParseMultipartForm(50 << 20); err != nil { // 50MB limit
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}

	files, err := service.ReadMultipartFiles(c.Request.MultipartForm)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req := service.PublishRequest{
		Slug:        c.PostForm("slug"),
		Version:     c.PostForm("version"),
		Changelog:   c.PostForm("changelog"),
		DisplayName: c.PostForm("displayName"),
		Summary:     c.PostForm("summary"),
		Files:       files,
	}
	if tags := c.PostForm("tags"); tags != "" {
		req.Tags = splitTags(tags)
	}

	skill, version, err := h.svc.PublishVersion(c.Request.Context(), user, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"skill":   skill,
		"version": version,
	})
}

// Delete handles DELETE /api/v1/skills/:slug
func (h *SkillHandler) Delete(c *gin.Context) {
	user := middleware.GetUser(c)
	slug := c.Param("slug")

	if err := h.svc.SoftDelete(c.Request.Context(), user, slug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "skill deleted"})
}

// Undelete handles POST /api/v1/skills/:slug/undelete
func (h *SkillHandler) Undelete(c *gin.Context) {
	user := middleware.GetUser(c)
	slug := c.Param("slug")

	if err := h.svc.Undelete(c.Request.Context(), user, slug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "skill restored"})
}

// Versions handles GET /api/v1/skills/:slug/versions
func (h *SkillHandler) Versions(c *gin.Context) {
	slug := c.Param("slug")
	versions, err := h.svc.GetVersions(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": versions})
}

// Version handles GET /api/v1/skills/:slug/versions/:version
func (h *SkillHandler) Version(c *gin.Context) {
	slug := c.Param("slug")
	ver := c.Param("version")

	version, err := h.svc.GetVersion(c.Request.Context(), slug, ver)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if version == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	c.JSON(http.StatusOK, version)
}

// GetFile handles GET /api/v1/skills/:slug/file
func (h *SkillHandler) GetFile(c *gin.Context) {
	slug := c.Param("slug")
	version := c.DefaultQuery("version", "latest")
	path := c.Query("path")
	if path == "" {
		path = "SKILL.md"
	}

	content, err := h.svc.GetFile(c.Request.Context(), slug, version, path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range splitAny(s, ",; ") {
		t = trimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func splitAny(s string, seps string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(seps, r)
	})
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
