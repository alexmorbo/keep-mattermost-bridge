package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/alexmorbo/keep-mattermost-bridge/interface/http/handler"
	"github.com/alexmorbo/keep-mattermost-bridge/interface/http/middleware"
)

func NewRouter(
	log *slog.Logger,
	webhookHandler *handler.WebhookHandler,
	callbackHandler *handler.CallbackHandlerHTTP,
	healthHandler *handler.HealthHandler,
) *gin.Engine {
	router := gin.New()

	// Recovery for all routes
	router.Use(middleware.Recovery(log))

	// Health endpoints — only recovery middleware
	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)
	router.GET("/metrics", healthHandler.Metrics)

	// API routes with full middleware stack
	v1 := router.Group("/api/v1")
	v1.Use(middleware.RequestID())
	v1.Use(middleware.BodyLimit(1 << 20))
	v1.Use(middleware.Metrics())
	v1.Use(middleware.Logging(log))
	{
		v1.POST("/webhook/alert", webhookHandler.HandleAlert)
		v1.POST("/callback", callbackHandler.HandleCallback)
	}

	return router
}
