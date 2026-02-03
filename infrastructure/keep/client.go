package keep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

var (
	keepEnrichOK    = metrics.NewCounter(`keep_api_calls_total{operation="enrich",status="ok"}`)
	keepEnrichErr   = metrics.NewCounter(`keep_api_calls_total{operation="enrich",status="error"}`)
	keepGetAlertOK  = metrics.NewCounter(`keep_api_calls_total{operation="get_alert",status="ok"}`)
	keepGetAlertErr = metrics.NewCounter(`keep_api_calls_total{operation="get_alert",status="error"}`)
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

type alertResponse struct {
	Fingerprint     string         `json:"fingerprint"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	Severity        string         `json:"severity"`
	Description     string         `json:"description"`
	Source          []string       `json:"source"`
	Labels          map[string]any `json:"labels"`
	FiringStartTime string         `json:"firingStartTime"`
	LastReceived    string         `json:"lastReceived"`
}

func (c *Client) GetAlert(ctx context.Context, fingerprint string) (*port.KeepAlert, error) {
	start := time.Now()
	reqURL := c.baseURL + "/api/alerts/" + url.PathEscape(fingerprint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Keep GetAlert failed",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", 0, duration, err.Error()),
		)
		keepGetAlertErr.Inc()
		return nil, fmt.Errorf("keep get alert: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep GetAlert non-200",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", resp.StatusCode, duration, string(respBody)),
		)
		keepGetAlertErr.Inc()
		return nil, fmt.Errorf("keep get alert: status %d, body: %s", resp.StatusCode, respBody)
	}

	var alertResp alertResponse
	if err := json.NewDecoder(resp.Body).Decode(&alertResp); err != nil {
		c.logger.Error("Keep GetAlert decode failed",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", resp.StatusCode, duration, err.Error()),
		)
		keepGetAlertErr.Inc()
		return nil, fmt.Errorf("decode alert response: %w", err)
	}

	c.logger.Debug("Keep GetAlert completed",
		logger.ExternalFields("keep", reqURL, "GET", resp.StatusCode, duration),
	)
	keepGetAlertOK.Inc()

	labels := make(map[string]string, len(alertResp.Labels))
	for k, v := range alertResp.Labels {
		if s, ok := v.(string); ok {
			labels[k] = s
		} else {
			labels[k] = fmt.Sprintf("%v", v)
		}
	}

	var firingStartTime time.Time
	if alertResp.FiringStartTime != "" {
		firingStartTime, _ = time.Parse(time.RFC3339, alertResp.FiringStartTime)
	}

	source := alertResp.Source
	if source == nil {
		source = []string{}
	}

	return &port.KeepAlert{
		Fingerprint:     alertResp.Fingerprint,
		Name:            alertResp.Name,
		Status:          alertResp.Status,
		Severity:        alertResp.Severity,
		Description:     alertResp.Description,
		Source:          source,
		Labels:          labels,
		FiringStartTime: firingStartTime,
	}, nil
}
