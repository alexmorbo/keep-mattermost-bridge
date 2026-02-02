package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/interface/http/handler"
)

func TestNewRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	require.NotNil(t, router)

	routes := router.Routes()
	require.NotEmpty(t, routes)

	routePaths := make(map[string]string)
	for _, route := range routes {
		if route.Path != "" {
			routePaths[route.Path] = route.Method
		}
	}

	assert.Contains(t, routePaths, "/health/live")
	assert.Contains(t, routePaths, "/health/ready")
	assert.Contains(t, routePaths, "/metrics")
	assert.Contains(t, routePaths, "/api/v1/webhook/alert")
	assert.Contains(t, routePaths, "/api/v1/callback")
}

func TestRouterHealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	tests := []struct {
		name   string
		path   string
		method string
	}{
		{
			name:   "health live endpoint",
			path:   "/health/live",
			method: http.MethodGet,
		},
		{
			name:   "health ready endpoint",
			path:   "/health/ready",
			method: http.MethodGet,
		},
		{
			name:   "metrics endpoint",
			path:   "/metrics",
			method: http.MethodGet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusNotFound, w.Code, "route should exist")
		})
	}
}

func TestRouterAPIv1Endpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	tests := []struct {
		name   string
		path   string
		method string
	}{
		{
			name:   "webhook alert endpoint",
			path:   "/api/v1/webhook/alert",
			method: http.MethodPost,
		},
		{
			name:   "callback endpoint",
			path:   "/api/v1/callback",
			method: http.MethodPost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusNotFound, w.Code, "route should exist")
		})
	}
}

func TestRouterMiddlewareOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alert", nil)

	router.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID, "RequestID middleware should set X-Request-ID header on API routes")
}

func TestRouterNotFoundRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRouterMethodNotAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health/live", nil)

	router.ServeHTTP(w, req)

	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed,
		"should return 404 or 405 for wrong HTTP method")
}

func TestRouterCreation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	webhookHandler := &handler.WebhookHandler{}
	callbackHandler := &handler.CallbackHandlerHTTP{}
	healthHandler := &handler.HealthHandler{}

	router := NewRouter(logger, webhookHandler, callbackHandler, healthHandler)

	require.NotNil(t, router)
}
