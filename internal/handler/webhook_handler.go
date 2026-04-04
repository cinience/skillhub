package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/internal/gitstore"
)

type WebhookHandler struct {
	importSvc *gitstore.ImportService
}

func NewWebhookHandler(importSvc *gitstore.ImportService) *WebhookHandler {
	return &WebhookHandler{importSvc: importSvc}
}

// GitHubWebhook handles POST /api/v1/webhooks/github
func (h *WebhookHandler) GitHubWebhook(c *gin.Context) {
	h.handleWebhook(c, "github", c.GetHeader("X-Hub-Signature-256"), gitstore.ParseGitHubWebhook)
}

// GitLabWebhook handles POST /api/v1/webhooks/gitlab
func (h *WebhookHandler) GitLabWebhook(c *gin.Context) {
	h.handleWebhook(c, "gitlab", c.GetHeader("X-Gitlab-Token"), gitstore.ParseGitLabWebhook)
}

// GiteaWebhook handles POST /api/v1/webhooks/gitea
func (h *WebhookHandler) GiteaWebhook(c *gin.Context) {
	h.handleWebhook(c, "gitea", c.GetHeader("X-Gitea-Signature"), gitstore.ParseGiteaWebhook)
}

func (h *WebhookHandler) handleWebhook(c *gin.Context, provider, signature string, parser func([]byte) (*gitstore.WebhookEvent, error)) {
	if !h.importSvc.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook import is not enabled"})
		return
	}

	// Limit webhook payload to 1 MB to prevent DoS
	payload, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	if !h.importSvc.VerifySignature(payload, signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	event, err := parser(payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "ignored: " + err.Error()})
		return
	}
	event.Provider = provider

	if err := h.importSvc.HandleWebhook(c.Request.Context(), event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "processed"})
}
