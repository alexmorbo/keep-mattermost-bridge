package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

func Logging(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		requestID := logger.GetRequestID(c.Request.Context())

		log.Info("HTTP request completed",
			logger.HTTPFields(
				requestID,
				method,
				path,
				c.ClientIP(),
				status,
				duration.Milliseconds(),
				int(c.Request.ContentLength),
				c.Writer.Size(),
			),
		)
	}
}
