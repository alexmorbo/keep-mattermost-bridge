package handler

import (
	"context"
	"net/http"

	"github.com/VictoriaMetrics/metrics"
	"github.com/gin-gonic/gin"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	postRepo HealthChecker
}

func NewHealthHandler(postRepo HealthChecker) *HealthHandler {
	return &HealthHandler{postRepo: postRepo}
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	if err := h.postRepo.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *HealthHandler) Metrics(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/plain")
	metrics.WritePrometheus(c.Writer, true)
}
