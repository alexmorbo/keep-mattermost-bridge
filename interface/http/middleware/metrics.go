package middleware

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/gin-gonic/gin"
)

var httpRequestsInFlight int64

func init() {
	metrics.NewGauge(`http_requests_in_flight`, func() float64 {
		return float64(atomic.LoadInt64(&httpRequestsInFlight))
	})
}

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		atomic.AddInt64(&httpRequestsInFlight, 1)
		defer atomic.AddInt64(&httpRequestsInFlight, -1)

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		path = strings.ReplaceAll(path, `"`, `_`)
		method = strings.ReplaceAll(method, `"`, `_`)

		labels := `handler="` + path + `",method="` + method + `",status="` + status + `"`

		metrics.GetOrCreateCounter(`http_requests_total{` + labels + `}`).Inc()
		metrics.GetOrCreateHistogram(`http_request_duration_seconds{handler="` + path + `",method="` + method + `"}`).Update(duration)
	}
}
