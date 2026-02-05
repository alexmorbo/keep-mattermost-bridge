package keep

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
)

func TestEnrichAlertSuccess(t *testing.T) {
	var capturedRequest enrichRequest
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/alerts/enrich", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		capturedAPIKey = r.Header.Get("X-API-KEY")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &capturedRequest)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-12345", map[string]string{"status": "acknowledged"})
	require.NoError(t, err)

	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "fp-12345", capturedRequest.Fingerprint)
	assert.Equal(t, "acknowledged", capturedRequest.Enrichments["status"])
}

func TestEnrichAlertSuccessWithCreatedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", map[string]string{"status": "resolved"})
	require.NoError(t, err)
}

func TestEnrichAlertSuccessWithNoContentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", map[string]string{"status": "firing"})
	require.NoError(t, err)
}

func TestEnrichAlertServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", map[string]string{"status": "acknowledged"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestEnrichAlertBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid fingerprint"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "", map[string]string{"status": "acknowledged"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid fingerprint")
}

func TestEnrichAlertNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", map[string]string{"status": "acknowledged"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keep enrich alert")
}

func TestEnrichAlertContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.EnrichAlert(ctx, "fp-123", map[string]string{"status": "acknowledged"})
	require.Error(t, err)
}

func TestEnrichAlertNilEnrichments(t *testing.T) {
	var capturedRequest enrichRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &capturedRequest)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", nil)
	require.NoError(t, err)

	assert.Equal(t, "fp-123", capturedRequest.Fingerprint)
	assert.NotNil(t, capturedRequest.Enrichments)
}

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("https://keep.example.com", "api-key-123", logger)

	require.NotNil(t, client)
	assert.Equal(t, "https://keep.example.com", client.baseURL)
	assert.Equal(t, "api-key-123", client.apiKey)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.logger)
}

func TestGetAlertSuccess(t *testing.T) {
	var capturedAPIKey string
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.Header.Get("X-API-KEY")
		assert.Equal(t, http.MethodGet, r.Method)

		resp := map[string]any{
			"fingerprint":     "fp-12345",
			"name":            "HighCPUUsage",
			"status":          "firing",
			"severity":        "critical",
			"description":     "CPU usage is above 90%",
			"source":          []string{"prometheus", "grafana"},
			"labels":          map[string]any{"host": "server1", "env": "prod"},
			"firingStartTime": "2024-01-15T10:30:00Z",
			"lastReceived":    "2024-01-15T10:35:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	alert, err := client.GetAlert(context.Background(), "fp-12345")
	require.NoError(t, err)
	require.NotNil(t, alert)

	assert.Equal(t, "/alerts/fp-12345", capturedPath)
	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "fp-12345", alert.Fingerprint)
	assert.Equal(t, "HighCPUUsage", alert.Name)
	assert.Equal(t, "firing", alert.Status)
	assert.Equal(t, "critical", alert.Severity)
	assert.Equal(t, "CPU usage is above 90%", alert.Description)
	assert.Equal(t, []string{"prometheus", "grafana"}, alert.Source)
	assert.Equal(t, map[string]string{"host": "server1", "env": "prod"}, alert.Labels)
	assert.Equal(t, 2024, alert.FiringStartTime.Year())
	assert.Equal(t, 1, int(alert.FiringStartTime.Month()))
	assert.Equal(t, 15, alert.FiringStartTime.Day())
}

func TestGetAlertNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	alert, err := client.GetAlert(context.Background(), "fp-123")
	require.Error(t, err)
	assert.Nil(t, alert)
	assert.Contains(t, err.Error(), "keep get alert")
}

func TestGetAlertNon200StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "alert not found"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	alert, err := client.GetAlert(context.Background(), "fp-nonexistent")
	require.Error(t, err)
	assert.Nil(t, alert)
	assert.Contains(t, err.Error(), "status 404")
	assert.Contains(t, err.Error(), "alert not found")
}

func TestGetAlertJSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	alert, err := client.GetAlert(context.Background(), "fp-123")
	require.Error(t, err)
	assert.Nil(t, alert)
	assert.Contains(t, err.Error(), "decode alert response")
}

func TestGetAlertEmptySource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"fingerprint": "fp-123",
			"name":        "TestAlert",
			"status":      "firing",
			"severity":    "warning",
			"description": "Test description",
			"source":      nil,
			"labels":      map[string]any{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	alert, err := client.GetAlert(context.Background(), "fp-123")
	require.NoError(t, err)
	require.NotNil(t, alert)
	assert.Equal(t, []string{}, alert.Source)
}

func TestGetAlertLabelsWithNonStringValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"fingerprint": "fp-123",
			"name":        "TestAlert",
			"status":      "firing",
			"severity":    "warning",
			"description": "Test description",
			"source":      []string{"test"},
			"labels": map[string]any{
				"string_val": "text",
				"int_val":    42,
				"float_val":  3.14,
				"bool_val":   true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	alert, err := client.GetAlert(context.Background(), "fp-123")
	require.NoError(t, err)
	require.NotNil(t, alert)

	assert.Equal(t, "text", alert.Labels["string_val"])
	assert.Equal(t, "42", alert.Labels["int_val"])
	assert.Equal(t, "3.14", alert.Labels["float_val"])
	assert.Equal(t, "true", alert.Labels["bool_val"])
}

func TestUnenrichAlertSuccess(t *testing.T) {
	var capturedRequest unenrichRequest
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/alerts/unenrich", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		capturedAPIKey = r.Header.Get("X-API-KEY")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &capturedRequest)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	err := client.UnenrichAlert(context.Background(), "fp-12345", []string{"status", "assignee"})
	require.NoError(t, err)

	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "fp-12345", capturedRequest.Fingerprint)
	assert.Equal(t, []string{"status", "assignee"}, capturedRequest.Enrichments)
}

func TestUnenrichAlertSuccessWithCreatedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.UnenrichAlert(context.Background(), "fp-123", []string{"status"})
	require.NoError(t, err)
}

func TestUnenrichAlertSuccessWithNoContentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.UnenrichAlert(context.Background(), "fp-123", []string{"status"})
	require.NoError(t, err)
}

func TestUnenrichAlertServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.UnenrichAlert(context.Background(), "fp-123", []string{"status"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestUnenrichAlertBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid fingerprint"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.UnenrichAlert(context.Background(), "", []string{"status"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid fingerprint")
}

func TestUnenrichAlertNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	err := client.UnenrichAlert(context.Background(), "fp-123", []string{"status"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keep unenrich alert")
}

func TestUnenrichAlertContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.UnenrichAlert(ctx, "fp-123", []string{"status"})
	require.Error(t, err)
}

func TestGetProvidersSuccess(t *testing.T) {
	var capturedAPIKey string
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.Header.Get("X-API-KEY")
		assert.Equal(t, http.MethodGet, r.Method)

		resp := map[string]any{
			"installed_providers": []map[string]any{
				{
					"id":   "provider-1",
					"type": "webhook",
					"details": map[string]any{
						"name": "mattermost-webhook",
						"url":  "https://example.com/webhook",
					},
				},
				{
					"id":   "provider-2",
					"type": "prometheus",
					"details": map[string]any{
						"name": "prometheus-prod",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	providers, err := client.GetProviders(context.Background())
	require.NoError(t, err)
	require.Len(t, providers, 2)

	assert.Equal(t, "/providers", capturedPath)
	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "provider-1", providers[0].ID)
	assert.Equal(t, "webhook", providers[0].Type)
	assert.Equal(t, "mattermost-webhook", providers[0].Name)
	assert.Equal(t, "provider-2", providers[1].ID)
	assert.Equal(t, "prometheus", providers[1].Type)
	assert.Equal(t, "prometheus-prod", providers[1].Name)
}

func TestGetProvidersEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"installed_providers": []map[string]any{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	providers, err := client.GetProviders(context.Background())
	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestGetProvidersNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	providers, err := client.GetProviders(context.Background())
	require.Error(t, err)
	assert.Nil(t, providers)
	assert.Contains(t, err.Error(), "keep get providers")
}

func TestGetProvidersNon200StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	providers, err := client.GetProviders(context.Background())
	require.Error(t, err)
	assert.Nil(t, providers)
	assert.Contains(t, err.Error(), "status 500")
}

func TestGetProvidersJSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	providers, err := client.GetProviders(context.Background())
	require.Error(t, err)
	assert.Nil(t, providers)
	assert.Contains(t, err.Error(), "decode providers response")
}

func TestGetProvidersMissingDetailsName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"installed_providers": []map[string]any{
				{
					"id":      "provider-1",
					"type":    "webhook",
					"details": map[string]any{},
				},
				{
					"id":      "provider-2",
					"type":    "prometheus",
					"details": nil,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	providers, err := client.GetProviders(context.Background())
	require.NoError(t, err)
	require.Len(t, providers, 2)
	assert.Equal(t, "", providers[0].Name)
	assert.Equal(t, "", providers[1].Name)
}

func TestCreateWebhookProviderSuccess(t *testing.T) {
	var capturedRequest webhookProviderRequest
	var capturedAPIKey string
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.Header.Get("X-API-KEY")
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &capturedRequest)
		require.NoError(t, err)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	config := port.WebhookProviderConfig{
		Name:   "mattermost-webhook",
		URL:    "https://example.com/webhook",
		Method: "POST",
		Verify: true,
	}
	err := client.CreateWebhookProvider(context.Background(), config)
	require.NoError(t, err)

	assert.Equal(t, "/providers/install", capturedPath)
	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "webhook", capturedRequest.ProviderType)
	assert.Equal(t, "mattermost-webhook", capturedRequest.ProviderID)
	assert.Equal(t, "mattermost-webhook", capturedRequest.ProviderName)
	assert.Equal(t, "https://example.com/webhook", capturedRequest.URL)
	assert.Equal(t, "POST", capturedRequest.Method)
	assert.True(t, capturedRequest.Verify)
}

func TestCreateWebhookProviderSuccessWithOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	config := port.WebhookProviderConfig{
		Name:   "test-webhook",
		URL:    "https://example.com",
		Method: "POST",
	}
	err := client.CreateWebhookProvider(context.Background(), config)
	require.NoError(t, err)
}

func TestCreateWebhookProviderNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	config := port.WebhookProviderConfig{
		Name: "test-webhook",
		URL:  "https://example.com",
	}
	err := client.CreateWebhookProvider(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keep create webhook provider")
}

func TestCreateWebhookProviderServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	config := port.WebhookProviderConfig{
		Name: "test-webhook",
		URL:  "https://example.com",
	}
	err := client.CreateWebhookProvider(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestCreateWebhookProviderBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid config"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	config := port.WebhookProviderConfig{
		Name: "test-webhook",
	}
	err := client.CreateWebhookProvider(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid config")
}

func TestGetWorkflowsSuccess(t *testing.T) {
	var capturedAPIKey string
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.Header.Get("X-API-KEY")
		assert.Equal(t, http.MethodGet, r.Method)

		resp := []map[string]any{
			{
				"id":              "workflow-1",
				"name":            "Alert Notification",
				"workflow_raw_id": "alert-notification-workflow",
				"disabled":        false,
			},
			{
				"id":              "workflow-2",
				"name":            "Escalation",
				"workflow_raw_id": "escalation-workflow",
				"disabled":        true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	workflows, err := client.GetWorkflows(context.Background())
	require.NoError(t, err)
	require.Len(t, workflows, 2)

	assert.Equal(t, "/workflows", capturedPath)
	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "workflow-1", workflows[0].ID)
	assert.Equal(t, "Alert Notification", workflows[0].Name)
	assert.Equal(t, "alert-notification-workflow", workflows[0].WorkflowRawID)
	assert.False(t, workflows[0].Disabled)
	assert.Equal(t, "workflow-2", workflows[1].ID)
	assert.Equal(t, "Escalation", workflows[1].Name)
	assert.Equal(t, "escalation-workflow", workflows[1].WorkflowRawID)
	assert.True(t, workflows[1].Disabled)
}

func TestGetWorkflowsEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	workflows, err := client.GetWorkflows(context.Background())
	require.NoError(t, err)
	assert.Empty(t, workflows)
}

func TestGetWorkflowsNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	workflows, err := client.GetWorkflows(context.Background())
	require.Error(t, err)
	assert.Nil(t, workflows)
	assert.Contains(t, err.Error(), "keep get workflows")
}

func TestGetWorkflowsNon200StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	workflows, err := client.GetWorkflows(context.Background())
	require.Error(t, err)
	assert.Nil(t, workflows)
	assert.Contains(t, err.Error(), "status 500")
}

func TestGetWorkflowsJSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	workflows, err := client.GetWorkflows(context.Background())
	require.Error(t, err)
	assert.Nil(t, workflows)
	assert.Contains(t, err.Error(), "decode workflows response")
}

func TestCreateWorkflowSuccess(t *testing.T) {
	var capturedAPIKey string
	var capturedPath string
	var capturedWorkflowContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.Header.Get("X-API-KEY")
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer func() { _ = file.Close() }()

		content, err := io.ReadAll(file)
		require.NoError(t, err)
		capturedWorkflowContent = string(content)

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-api-key", logger)

	config := port.WorkflowConfig{
		ID:          "mattermost-notification",
		Name:        "Mattermost Notification",
		Description: "Send alerts to Mattermost",
		Workflow:    "workflow:\n  id: test\n  steps: []",
	}
	err := client.CreateWorkflow(context.Background(), config)
	require.NoError(t, err)

	assert.Equal(t, "/workflows", capturedPath)
	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "workflow:\n  id: test\n  steps: []", capturedWorkflowContent)
}

func TestCreateWorkflowSuccessWithOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	config := port.WorkflowConfig{
		Workflow: "workflow: {}",
	}
	err := client.CreateWorkflow(context.Background(), config)
	require.NoError(t, err)
}

func TestCreateWorkflowNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	config := port.WorkflowConfig{
		Workflow: "workflow: {}",
	}
	err := client.CreateWorkflow(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keep create workflow")
}

func TestCreateWorkflowServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	config := port.WorkflowConfig{
		Workflow: "workflow: {}",
	}
	err := client.CreateWorkflow(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestCreateWorkflowBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid workflow yaml"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	config := port.WorkflowConfig{
		Workflow: "invalid yaml",
	}
	err := client.CreateWorkflow(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid workflow yaml")
}
