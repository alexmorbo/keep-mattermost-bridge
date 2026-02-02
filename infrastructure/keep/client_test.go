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
