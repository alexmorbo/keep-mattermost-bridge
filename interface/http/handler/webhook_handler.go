package handler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
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
	logger      *slog.Logger
}

func NewWebhookHandler(handleAlert AlertHandler, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{handleAlert: handleAlert, logger: logger}
}

func (h *WebhookHandler) HandleAlert(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	h.logger.Info("Incoming webhook payload", slog.String("body", string(body)))

	var input dto.KeepAlertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Error("Failed to parse webhook payload", slog.String("error", err.Error()))
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
