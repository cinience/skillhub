package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/saker-ai/skillhub/pkg/middleware"
	"github.com/saker-ai/skillhub/pkg/model"
	"github.com/saker-ai/skillhub/pkg/repository"
	"github.com/saker-ai/skillhub/pkg/service"
)

type PluginHandler struct {
	svc *service.PluginService
}

func NewPluginHandler(svc *service.PluginService) *PluginHandler {
	return &PluginHandler{svc: svc}
}

func extractPluginRef(c *gin.Context) string {
	ns := c.Param("namespace")
	slug := c.Param("slug")
	if ns != "" {
		return "@" + strings.TrimPrefix(ns, "@") + "/" + slug
	}
	return slug
}

func pluginQueryRef(c *gin.Context) string {
	slug := c.Query("slug")
	if ns := c.Query("namespace"); ns != "" {
		return "@" + ns + "/" + slug
	}
	return slug
}

// POST /api/v1/plugins
func (h *PluginHandler) Publish(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form: " + err.Error()})
		return
	}

	input, err := h.svc.ParseMultipartPublish(c.Request.MultipartForm)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	input.OwnerID = user.ID
	input.User = user
	input.TokenNamespace = middleware.GetTokenNamespace(c)
	if ns := c.PostForm("namespace"); ns != "" {
		input.NamespaceSlug = ns
	}

	result, err := h.svc.Publish(c.Request.Context(), *input)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"plugin":  result.Plugin,
		"version": result.Version,
	})
}

// GET /api/v1/plugins
func (h *PluginHandler) List(c *gin.Context) {
	opts := repository.PluginListOptions{
		Category: c.Query("category"),
		Sort:     c.Query("sort"),
		Cursor:   c.Query("cursor"),
		Limit:    queryInt(c, "limit", 20),
	}

	plugins, nextCursor, err := h.svc.List(c.Request.Context(), opts)
	if err != nil {
		writeInternalError(c, "list_plugins", err)
		return
	}

	data := make([]gin.H, 0, len(plugins))
	for _, p := range plugins {
		version, err := h.svc.LatestVersion(c.Request.Context(), p.ID)
		if err != nil {
			writeInternalError(c, "resolve_plugin_version", err)
			return
		}
		data = append(data, pluginToJSON(p, version))
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       data,
		"nextCursor": nextCursor,
	})
}

// GET /api/v1/plugins/:slug
func (h *PluginHandler) Get(c *gin.Context) {
	p, err := h.svc.Get(c.Request.Context(), extractPluginRef(c), middleware.GetUser(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	version, err := h.svc.LatestVersion(c.Request.Context(), p.ID)
	if err != nil {
		writeInternalError(c, "resolve_plugin_version", err)
		return
	}
	c.JSON(http.StatusOK, pluginToJSON(*p, version))
}

// GET /api/v1/plugins/by-id/:id
func (h *PluginHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plugin id"})
		return
	}
	p, err := h.svc.GetByID(c.Request.Context(), id, middleware.GetUser(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	version, err := h.svc.LatestVersion(c.Request.Context(), p.ID)
	if err != nil {
		writeInternalError(c, "resolve_plugin_version", err)
		return
	}
	c.JSON(http.StatusOK, pluginToJSON(*p, version))
}

// GET /api/v1/plugins/:slug/versions
func (h *PluginHandler) Versions(c *gin.Context) {
	versions, err := h.svc.Versions(c.Request.Context(), extractPluginRef(c), middleware.GetUser(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

// GET /api/v1/plugins/file?slug=x&version=y&path=z
func (h *PluginHandler) GetFile(c *gin.Context) {
	ref := pluginQueryRef(c)
	version := c.Query("version")
	filePath := c.Query("path")

	if ref == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug and path are required"})
		return
	}

	data, err := h.svc.GetFile(c.Request.Context(), ref, version, filePath, middleware.GetUser(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Writer.Write(data)
}

// GET /api/v1/plugins/download?slug=x&version=y
func (h *PluginHandler) Download(c *gin.Context) {
	ref := pluginQueryRef(c)
	version := c.Query("version")

	if ref == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}

	reader, etag, err := h.svc.Download(c.Request.Context(), ref, version, middleware.GetUser(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	defer reader.Close()

	if etag != "" {
		c.Header("ETag", etag)
		if c.GetHeader("If-None-Match") == etag {
			c.Status(http.StatusNotModified)
			return
		}
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", c.Query("slug")+".zip"))
	io.Copy(c.Writer, reader)
}

func pluginToJSON(p model.PluginWithOwner, version *model.PluginVersion) gin.H {
	result := gin.H{
		"id":            p.ID,
		"slug":          p.Slug,
		"visibility":    p.Visibility,
		"category":      p.Category,
		"tags":          p.Tags,
		"downloads":     p.Downloads,
		"starsCount":    p.StarsCount,
		"ownerHandle":   p.OwnerHandle,
		"namespaceSlug": p.NamespaceSlug,
		"createdAt":     p.CreatedAt,
		"updatedAt":     p.UpdatedAt,
	}
	if p.DisplayName != nil {
		result["displayName"] = *p.DisplayName
	}
	if p.Summary != nil {
		result["summary"] = *p.Summary
	}
	if p.LatestVersionID != nil {
		result["latestVersionId"] = *p.LatestVersionID
	}
	if version != nil {
		result["version"] = version.Version
		result["fingerprint"] = version.Fingerprint
		result["sha256Hash"] = version.SHA256Hash
		var files []struct {
			Path   string `json:"path"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		}
		if err := json.Unmarshal(version.Files, &files); err == nil {
			result["files"] = files
		}
	}

	return result
}

func (h *PluginHandler) Delete(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if err := h.svc.SoftDelete(c.Request.Context(), user, extractPluginRef(c)); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "plugin deleted"})
}

func (h *PluginHandler) Undelete(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if err := h.svc.Undelete(c.Request.Context(), user, extractPluginRef(c)); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "plugin restored"})
}

func (h *PluginHandler) YankVersion(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	version := c.Param("version")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.svc.YankVersion(c.Request.Context(), user, extractPluginRef(c), version, body.Reason); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "version yanked"})
}

func (h *PluginHandler) UnyankVersion(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	version := c.Param("version")
	if err := h.svc.UnyankVersion(c.Request.Context(), user, extractPluginRef(c), version); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "version unyanked"})
}

func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	if n > 100 {
		n = 100
	}
	return n
}
