package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
)

type AlertHandler interface {
	Execute(ctx context.Context, input dto.KeepAlertInput) error
}

type WebhookHandler struct {
	handleAlert AlertHandler
}

func NewWebhookHandler(handleAlert AlertHandler) *WebhookHandler {
	return &WebhookHandler{handleAlert: handleAlert}
}

func (h *WebhookHandler) HandleAlert(c *gin.Context) {
	var input dto.KeepAlertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.handleAlert.Execute(ctx, input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
