package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
)

type mockAlertExecutor struct {
	executeFunc func(ctx context.Context, input dto.KeepAlertInput) error
}

func (m *mockAlertExecutor) Execute(ctx context.Context, input dto.KeepAlertInput) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return nil
}

type mockCallbackExecutor struct {
	executeFunc func(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error)
}

func (m *mockCallbackExecutor) Execute(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return &dto.CallbackOutput{}, nil
}

type mockPostRepositoryPinger struct {
	pingFunc func(ctx context.Context) error
}

func (m *mockPostRepositoryPinger) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestWebhookHandlerValidJSON(t *testing.T) {
	called := false
	mockUseCase := &mockAlertExecutor{
		executeFunc: func(ctx context.Context, input dto.KeepAlertInput) error {
			called = true
			assert.Equal(t, "test-alert", input.Name)
			assert.Equal(t, "critical", input.Severity)
			assert.Equal(t, "firing", input.Status)
			return nil
		},
	}

	handler := &WebhookHandler{handleAlert: mockUseCase}

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleAlert)

	alertInput := dto.KeepAlertInput{
		ID:          "alert-123",
		Name:        "test-alert",
		Status:      "firing",
		Severity:    "critical",
		Source:      []string{"prometheus"},
		Fingerprint: "abc123",
		Description: "Test alert description",
		Labels:      map[string]string{},
	}

	body, err := json.Marshal(alertInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called, "use case should have been called")

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestWebhookHandlerInvalidJSON(t *testing.T) {
	mockUseCase := &mockAlertExecutor{}
	handler := &WebhookHandler{handleAlert: mockUseCase}

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleAlert)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", bytes.NewBufferString("invalid json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", response["error"])
}

func TestWebhookHandlerUseCaseError(t *testing.T) {
	mockUseCase := &mockAlertExecutor{
		executeFunc: func(ctx context.Context, input dto.KeepAlertInput) error {
			return errors.New("use case execution failed")
		},
	}

	handler := &WebhookHandler{handleAlert: mockUseCase}

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleAlert)

	alertInput := dto.KeepAlertInput{
		ID:          "alert-123",
		Name:        "test-alert",
		Status:      "firing",
		Severity:    "critical",
		Fingerprint: "abc123",
	}

	body, err := json.Marshal(alertInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "internal error", response["error"])
}

func TestCallbackHandlerValidJSON(t *testing.T) {
	expectedOutput := &dto.CallbackOutput{
		Attachment: dto.AttachmentDTO{
			Color: "#00CC00",
			Title: "Resolved Alert",
		},
		Ephemeral: "Alert resolved",
	}

	called := false
	mockUseCase := &mockCallbackExecutor{
		executeFunc: func(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
			called = true
			assert.Equal(t, "user-123", input.UserID)
			assert.Equal(t, "resolve", input.Context["action"])
			return expectedOutput, nil
		},
	}

	handler := &CallbackHandlerHTTP{handleCallback: mockUseCase}

	router := setupTestRouter()
	router.POST("/callback", handler.HandleCallback)

	callbackInput := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "abc123",
			"alert_name":  "test-alert",
		},
	}

	body, err := json.Marshal(callbackInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/callback", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called, "use case should have been called")

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response["update"])
	assert.Equal(t, "Alert resolved", response["ephemeral_text"])

	update := response["update"].(map[string]interface{})
	props := update["props"].(map[string]interface{})
	attachments := props["attachments"].([]interface{})
	assert.Len(t, attachments, 1)
}

func TestCallbackHandlerInvalidJSON(t *testing.T) {
	mockUseCase := &mockCallbackExecutor{}
	handler := &CallbackHandlerHTTP{handleCallback: mockUseCase}

	router := setupTestRouter()
	router.POST("/callback", handler.HandleCallback)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/callback", bytes.NewBufferString("invalid json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", response["error"])
}

func TestCallbackHandlerUseCaseError(t *testing.T) {
	mockUseCase := &mockCallbackExecutor{
		executeFunc: func(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
			return nil, errors.New("callback execution failed")
		},
	}

	handler := &CallbackHandlerHTTP{handleCallback: mockUseCase}

	router := setupTestRouter()
	router.POST("/callback", handler.HandleCallback)

	callbackInput := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action": "resolve",
		},
	}

	body, err := json.Marshal(callbackInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/callback", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "internal error", response["error"])
}

func TestHealthHandlerLive(t *testing.T) {
	mockRepo := &mockPostRepositoryPinger{}
	handler := &HealthHandler{postRepo: mockRepo}

	router := setupTestRouter()
	router.GET("/health/live", handler.Live)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/health/live", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestHealthHandlerReadyHealthy(t *testing.T) {
	mockRepo := &mockPostRepositoryPinger{
		pingFunc: func(ctx context.Context) error {
			return nil
		},
	}
	handler := &HealthHandler{postRepo: mockRepo}

	router := setupTestRouter()
	router.GET("/health/ready", handler.Ready)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/health/ready", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ready", response["status"])
}

func TestHealthHandlerReadyUnhealthy(t *testing.T) {
	mockRepo := &mockPostRepositoryPinger{
		pingFunc: func(ctx context.Context) error {
			return errors.New("valkey connection failed")
		},
	}
	handler := &HealthHandler{postRepo: mockRepo}

	router := setupTestRouter()
	router.GET("/health/ready", handler.Ready)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/health/ready", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "not ready", response["status"])
	_, hasError := response["error"]
	assert.False(t, hasError, "error field should not be present in response")
}

func TestHealthHandlerMetrics(t *testing.T) {
	mockRepo := &mockPostRepositoryPinger{}
	handler := &HealthHandler{postRepo: mockRepo}

	router := setupTestRouter()
	router.GET("/metrics", handler.Metrics)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.String(), "metrics should not be empty")
}

func TestCallbackHandlerAcknowledgeAction(t *testing.T) {
	mockUseCase := &mockCallbackExecutor{
		executeFunc: func(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
			assert.Equal(t, "acknowledge", input.Context["action"])
			return &dto.CallbackOutput{
				Attachment: dto.AttachmentDTO{
					Color: "#FFA500",
					Title: "Acknowledged Alert",
				},
				Ephemeral: "Alert acknowledged by @user",
			}, nil
		},
	}

	handler := &CallbackHandlerHTTP{handleCallback: mockUseCase}

	router := setupTestRouter()
	router.POST("/callback", handler.HandleCallback)

	callbackInput := dto.MattermostCallbackInput{
		UserID: "user-456",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "xyz789",
			"alert_name":  "cpu-alert",
		},
	}

	body, err := json.Marshal(callbackInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/callback", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Alert acknowledged by @user", response["ephemeral_text"])
}

func TestWebhookHandlerMissingFields(t *testing.T) {
	mockUseCase := &mockAlertExecutor{
		executeFunc: func(ctx context.Context, input dto.KeepAlertInput) error {
			return nil
		},
	}

	handler := &WebhookHandler{handleAlert: mockUseCase}

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleAlert)

	alertInput := map[string]string{
		"name": "test-alert",
	}

	body, err := json.Marshal(alertInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", response["error"])
}

func TestWebhookHandlerEmptyBody(t *testing.T) {
	mockUseCase := &mockAlertExecutor{}
	handler := &WebhookHandler{handleAlert: mockUseCase}

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleAlert)

	body, err := json.Marshal(map[string]string{})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", response["error"])
}

func TestCallbackHandlerMissingContext(t *testing.T) {
	mockUseCase := &mockCallbackExecutor{
		executeFunc: func(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
			assert.NotNil(t, input.Context)
			return &dto.CallbackOutput{Ephemeral: "processed"}, nil
		},
	}

	handler := &CallbackHandlerHTTP{handleCallback: mockUseCase}

	router := setupTestRouter()
	router.POST("/callback", handler.HandleCallback)

	callbackInput := dto.MattermostCallbackInput{
		UserID:  "user-789",
		Context: map[string]string{},
	}

	body, err := json.Marshal(callbackInput)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/callback", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
