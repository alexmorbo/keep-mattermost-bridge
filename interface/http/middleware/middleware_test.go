package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

func TestRequestIDGeneratesUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, c.Request)

	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID)
}

func TestRequestIDUsesExistingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Request.Header.Set("X-Request-ID", "existing-request-id-123")

	RequestID()(c)

	requestID := c.GetHeader("X-Request-ID")
	assert.Equal(t, "existing-request-id-123", requestID)

	ctxRequestID := logger.GetRequestID(c.Request.Context())
	assert.Equal(t, "existing-request-id-123", ctxRequestID)
}

func TestLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.Use(RequestID())
	router.Use(Logging(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test response")
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	router.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func TestLoggingMiddlewareWithDifferentStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		handlerFunc  func(c *gin.Context)
	}{
		{
			name:       "200 OK",
			statusCode: http.StatusOK,
			handlerFunc: func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			},
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			handlerFunc: func(c *gin.Context) {
				c.String(http.StatusNotFound, "not found")
			},
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			handlerFunc: func(c *gin.Context) {
				c.String(http.StatusInternalServerError, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)

			router.Use(Logging(logger))
			router.GET("/test", tt.handlerFunc)

			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
			router.ServeHTTP(w, c.Request)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.Use(Recovery(logger))
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

func TestRecoveryMiddlewareDoesNotAffectNormalRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.Use(Recovery(logger))
	router.GET("/normal", func(c *gin.Context) {
		c.String(http.StatusOK, "normal response")
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/normal", nil)
	router.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "normal response", w.Body.String())
}

func TestMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.Use(Metrics())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "metrics test")
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "metrics test", w.Body.String())
}

func TestMetricsMiddlewareWithDifferentMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)

			router.Use(Metrics())
			router.Handle(method, "/test", func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			c.Request = httptest.NewRequest(method, "/test", nil)
			router.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestAllMiddlewaresTogether(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	w := httptest.NewRecorder()
	c, router := gin.CreateTestContext(w)

	router.Use(Recovery(logger))
	router.Use(RequestID())
	router.Use(Metrics())
	router.Use(Logging(logger))

	router.GET("/integrated", func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		c.String(http.StatusOK, "request-id: "+requestID)
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/integrated", nil)
	router.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "request-id:")
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}
