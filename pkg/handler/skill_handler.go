package handler

import (
	"log"
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

	category := c.Query("category")
	viewer := middleware.GetUser(c)
	skills, nextCursor, err := h.svc.ListSkills(c.Request.Context(), limit, cursor, sort, category, viewer)
	if err != nil {
		log.Printf("ListSkills error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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
	viewer := middleware.GetUser(c)
	skill, err := h.svc.GetSkill(c.Request.Context(), slug, viewer)
	if err != nil {
		log.Printf("GetSkill error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
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
		Category:    c.PostForm("category"),
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
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
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
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
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
	viewer := middleware.GetUser(c)
	versions, err := h.svc.GetVersions(c.Request.Context(), slug, viewer)
	if err != nil {
		log.Printf("GetVersions error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

// Version handles GET /api/v1/skills/:slug/versions/:version
func (h *SkillHandler) Version(c *gin.Context) {
	slug := c.Param("slug")
	ver := c.Param("version")
	viewer := middleware.GetUser(c)

	version, err := h.svc.GetVersion(c.Request.Context(), slug, ver, viewer)
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
	viewer := middleware.GetUser(c)

	content, err := h.svc.GetFile(c.Request.Context(), slug, version, path, viewer)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

// RequestPublic handles POST /api/v1/skills/:slug/request-public
func (h *SkillHandler) RequestPublic(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	slug := c.Param("slug")

	if err := h.svc.RequestPublic(c.Request.Context(), user, slug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "public review requested"})
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(",; ", r)
	}) {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
