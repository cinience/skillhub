package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/auth"
	"github.com/cinience/skillhub/pkg/middleware"
)

type DeviceAuthHandler struct {
	deviceSvc *auth.DeviceAuthService
}

func NewDeviceAuthHandler(deviceSvc *auth.DeviceAuthService) *DeviceAuthHandler {
	return &DeviceAuthHandler{deviceSvc: deviceSvc}
}

// RequestCode handles POST /auth/device/code — CLI requests a device code.
func (h *DeviceAuthHandler) RequestCode(c *gin.Context) {
	resp, err := h.deviceSvc.CreateCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// VerifyPage handles GET /auth/device/verify — shows the verification page.
func (h *DeviceAuthHandler) VerifyPage(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login?redirect=/auth/device/verify")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Enter your device code to authorize the CLI.",
	})
}

// VerifySubmit handles POST /auth/device/verify — user submits the user code.
func (h *DeviceAuthHandler) VerifySubmit(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		UserCode string `json:"userCode" form:"userCode"`
	}
	if err := c.ShouldBind(&req); err != nil || req.UserCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userCode is required"})
		return
	}

	userCode := auth.FormatUserCode(req.UserCode)

	if err := h.deviceSvc.Authorize(c.Request.Context(), userCode, user.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device authorized successfully."})
}

// PollToken handles POST /auth/device/token — CLI polls for the token.
func (h *DeviceAuthHandler) PollToken(c *gin.Context) {
	var req struct {
		DeviceCode string `json:"deviceCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DeviceCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceCode is required"})
		return
	}

	token, err := h.deviceSvc.PollToken(req.DeviceCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if token == "pending" {
		c.JSON(http.StatusAccepted, gin.H{"status": "pending"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
