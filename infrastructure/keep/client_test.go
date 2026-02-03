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
)

func TestEnrichAlertSuccess(t *testing.T) {
	var capturedRequest enrichRequest
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/alerts/enrich", r.URL.Path)
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

	err := client.EnrichAlert(context.Background(), "fp-12345", "acknowledged")
	require.NoError(t, err)

	assert.Equal(t, "test-api-key", capturedAPIKey)
	assert.Equal(t, "fp-12345", capturedRequest.Fingerprint)
	assert.Equal(t, "acknowledged", capturedRequest.Status)
}

func TestEnrichAlertSuccessWithCreatedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", "resolved")
	require.NoError(t, err)
}

func TestEnrichAlertSuccessWithNoContentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient(server.URL, "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", "firing")
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

	err := client.EnrichAlert(context.Background(), "fp-123", "acknowledged")
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

	err := client.EnrichAlert(context.Background(), "", "acknowledged")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid fingerprint")
}

func TestEnrichAlertNetworkError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := NewClient("http://localhost:1", "test-key", logger)

	err := client.EnrichAlert(context.Background(), "fp-123", "acknowledged")
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

	err := client.EnrichAlert(ctx, "fp-123", "acknowledged")
	require.Error(t, err)
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

	assert.Equal(t, "/api/alerts/fp-12345", capturedPath)
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
