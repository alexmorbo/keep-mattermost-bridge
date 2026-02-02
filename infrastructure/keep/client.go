package keep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

var (
	keepEnrichOK  = metrics.NewCounter(`keep_api_calls_total{operation="enrich",status="ok"}`)
	keepEnrichErr = metrics.NewCounter(`keep_api_calls_total{operation="enrich",status="error"}`)
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(baseURL, apiKey string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: logger,
	}
}

type enrichRequest struct {
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
}

func (c *Client) EnrichAlert(ctx context.Context, fingerprint, status string) error {
	start := time.Now()
	reqURL := c.baseURL + "/api/alerts/enrich"

	body := enrichRequest{
		Fingerprint: fingerprint,
		Status:      status,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal enrich body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Keep EnrichAlert failed",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", 0, duration, err.Error()),
		)
		keepEnrichErr.Inc()
		return fmt.Errorf("keep enrich alert: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep EnrichAlert non-2xx",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", resp.StatusCode, duration, string(respBody)),
		)
		keepEnrichErr.Inc()
		return fmt.Errorf("keep enrich alert: status %d, body: %s", resp.StatusCode, respBody)
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	c.logger.Debug("Keep EnrichAlert completed",
		logger.ExternalFields("keep", reqURL, "POST", resp.StatusCode, duration),
	)
	keepEnrichOK.Inc()

	return nil
}
