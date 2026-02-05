package keep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

var (
	keepEnrichOK          = metrics.NewCounter(`keep_api_calls_total{operation="enrich",status="ok"}`)
	keepEnrichErr         = metrics.NewCounter(`keep_api_calls_total{operation="enrich",status="error"}`)
	keepUnenrichOK        = metrics.NewCounter(`keep_api_calls_total{operation="unenrich",status="ok"}`)
	keepUnenrichErr       = metrics.NewCounter(`keep_api_calls_total{operation="unenrich",status="error"}`)
	keepGetAlertOK        = metrics.NewCounter(`keep_api_calls_total{operation="get_alert",status="ok"}`)
	keepGetAlertErr       = metrics.NewCounter(`keep_api_calls_total{operation="get_alert",status="error"}`)
	keepGetProvidersOK    = metrics.NewCounter(`keep_api_calls_total{operation="get_providers",status="ok"}`)
	keepGetProvidersErr   = metrics.NewCounter(`keep_api_calls_total{operation="get_providers",status="error"}`)
	keepCreateProviderOK  = metrics.NewCounter(`keep_api_calls_total{operation="create_provider",status="ok"}`)
	keepCreateProviderErr = metrics.NewCounter(`keep_api_calls_total{operation="create_provider",status="error"}`)
	keepGetWorkflowsOK    = metrics.NewCounter(`keep_api_calls_total{operation="get_workflows",status="ok"}`)
	keepGetWorkflowsErr   = metrics.NewCounter(`keep_api_calls_total{operation="get_workflows",status="error"}`)
	keepCreateWorkflowOK  = metrics.NewCounter(`keep_api_calls_total{operation="create_workflow",status="ok"}`)
	keepCreateWorkflowErr = metrics.NewCounter(`keep_api_calls_total{operation="create_workflow",status="error"}`)
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
	Fingerprint string            `json:"fingerprint"`
	Enrichments map[string]string `json:"enrichments"`
}

type unenrichRequest struct {
	Fingerprint string   `json:"fingerprint"`
	Enrichments []string `json:"enrichments"`
}

func (c *Client) EnrichAlert(ctx context.Context, fingerprint string, enrichments map[string]string, opts port.EnrichOptions) error {
	if enrichments == nil {
		enrichments = make(map[string]string)
	}

	start := time.Now()
	reqURL := c.baseURL + "/alerts/enrich"
	if opts.DisposeOnNewAlert {
		reqURL += "?dispose_on_new_alert=true"
	}

	body := enrichRequest{
		Fingerprint: fingerprint,
		Enrichments: enrichments,
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

func (c *Client) UnenrichAlert(ctx context.Context, fingerprint string, enrichments []string) error {
	if enrichments == nil {
		enrichments = []string{}
	}

	start := time.Now()
	reqURL := c.baseURL + "/alerts/unenrich"

	body := unenrichRequest{
		Fingerprint: fingerprint,
		Enrichments: enrichments,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal unenrich body: %w", err)
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
		c.logger.Error("Keep UnenrichAlert failed",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", 0, duration, err.Error()),
		)
		keepUnenrichErr.Inc()
		return fmt.Errorf("keep unenrich alert: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep UnenrichAlert non-2xx",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", resp.StatusCode, duration, string(respBody)),
		)
		keepUnenrichErr.Inc()
		return fmt.Errorf("keep unenrich alert: status %d, body: %s", resp.StatusCode, respBody)
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	c.logger.Debug("Keep UnenrichAlert completed",
		logger.ExternalFields("keep", reqURL, "POST", resp.StatusCode, duration),
	)
	keepUnenrichOK.Inc()

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
	Enrichments     map[string]any `json:"enrichments"`
	FiringStartTime string         `json:"firingStartTime"`
	LastReceived    string         `json:"lastReceived"`
	// Keep API returns assignee as top-level field, not inside enrichments
	Assignee string `json:"assignee"`
}

func (c *Client) GetAlert(ctx context.Context, fingerprint string) (*port.KeepAlert, error) {
	start := time.Now()
	reqURL := c.baseURL + "/alerts/" + url.PathEscape(fingerprint)

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
		var parseErr error
		firingStartTime, parseErr = time.Parse(time.RFC3339, alertResp.FiringStartTime)
		if parseErr != nil {
			c.logger.Debug("Failed to parse firingStartTime from Keep API",
				slog.String("value", alertResp.FiringStartTime),
				slog.String("error", parseErr.Error()),
			)
		}
	}

	source := alertResp.Source
	if source == nil {
		source = []string{}
	}

	enrichments := make(map[string]string)
	for k, v := range alertResp.Enrichments {
		if str, ok := v.(string); ok {
			enrichments[k] = str
		} else if v != nil {
			enrichments[k] = fmt.Sprintf("%v", v)
		}
	}
	// Keep API returns assignee as top-level field, add it to enrichments for consistency
	if alertResp.Assignee != "" {
		enrichments["assignee"] = alertResp.Assignee
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
		Enrichments:     enrichments,
	}, nil
}

type providersResponse struct {
	InstalledProviders []providerResponse `json:"installed_providers"`
}

type providerResponse struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Details map[string]any `json:"details"`
}

func (c *Client) GetProviders(ctx context.Context) ([]port.KeepProvider, error) {
	start := time.Now()
	reqURL := c.baseURL + "/providers"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Keep GetProviders failed",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", 0, duration, err.Error()),
		)
		keepGetProvidersErr.Inc()
		return nil, fmt.Errorf("keep get providers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep GetProviders non-200",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", resp.StatusCode, duration, string(respBody)),
		)
		keepGetProvidersErr.Inc()
		return nil, fmt.Errorf("keep get providers: status %d, body: %s", resp.StatusCode, respBody)
	}

	var providersResp providersResponse
	if err := json.NewDecoder(resp.Body).Decode(&providersResp); err != nil {
		c.logger.Error("Keep GetProviders decode failed",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", resp.StatusCode, duration, err.Error()),
		)
		keepGetProvidersErr.Inc()
		return nil, fmt.Errorf("decode providers response: %w", err)
	}

	c.logger.Debug("Keep GetProviders completed",
		logger.ExternalFields("keep", reqURL, "GET", resp.StatusCode, duration),
	)
	keepGetProvidersOK.Inc()

	providers := make([]port.KeepProvider, 0, len(providersResp.InstalledProviders))
	for _, p := range providersResp.InstalledProviders {
		name := ""
		if p.Details != nil {
			if n, ok := p.Details["name"].(string); ok {
				name = n
			}
		}
		providers = append(providers, port.KeepProvider{
			ID:      p.ID,
			Type:    p.Type,
			Name:    name,
			Details: p.Details,
		})
	}

	return providers, nil
}

type webhookProviderRequest struct {
	ProviderType string `json:"provider_type"`
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	URL          string `json:"url"`
	Method       string `json:"method"`
	Verify       bool   `json:"verify"`
}

func (c *Client) CreateWebhookProvider(ctx context.Context, config port.WebhookProviderConfig) error {
	start := time.Now()
	reqURL := c.baseURL + "/providers/install"

	body := webhookProviderRequest{
		ProviderType: "webhook",
		ProviderID:   config.Name,
		ProviderName: config.Name,
		URL:          config.URL,
		Method:       config.Method,
		Verify:       config.Verify,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal webhook provider body: %w", err)
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
		c.logger.Error("Keep CreateWebhookProvider failed",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", 0, duration, err.Error()),
		)
		keepCreateProviderErr.Inc()
		return fmt.Errorf("keep create webhook provider: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep CreateWebhookProvider non-2xx",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", resp.StatusCode, duration, string(respBody)),
		)
		keepCreateProviderErr.Inc()
		return fmt.Errorf("keep create webhook provider: status %d, body: %s", resp.StatusCode, respBody)
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	c.logger.Debug("Keep CreateWebhookProvider completed",
		logger.ExternalFields("keep", reqURL, "POST", resp.StatusCode, duration),
	)
	keepCreateProviderOK.Inc()

	return nil
}

type workflowResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	WorkflowRawID string `json:"workflow_raw_id"`
	Disabled      bool   `json:"disabled"`
}

func (c *Client) GetWorkflows(ctx context.Context) ([]port.KeepWorkflow, error) {
	start := time.Now()
	reqURL := c.baseURL + "/workflows"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Keep GetWorkflows failed",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", 0, duration, err.Error()),
		)
		keepGetWorkflowsErr.Inc()
		return nil, fmt.Errorf("keep get workflows: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep GetWorkflows non-200",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", resp.StatusCode, duration, string(respBody)),
		)
		keepGetWorkflowsErr.Inc()
		return nil, fmt.Errorf("keep get workflows: status %d, body: %s", resp.StatusCode, respBody)
	}

	var workflowsResp []workflowResponse
	if err := json.NewDecoder(resp.Body).Decode(&workflowsResp); err != nil {
		c.logger.Error("Keep GetWorkflows decode failed",
			logger.ExternalFieldsWithError("keep", reqURL, "GET", resp.StatusCode, duration, err.Error()),
		)
		keepGetWorkflowsErr.Inc()
		return nil, fmt.Errorf("decode workflows response: %w", err)
	}

	c.logger.Debug("Keep GetWorkflows completed",
		logger.ExternalFields("keep", reqURL, "GET", resp.StatusCode, duration),
	)
	keepGetWorkflowsOK.Inc()

	workflows := make([]port.KeepWorkflow, 0, len(workflowsResp))
	for _, w := range workflowsResp {
		workflows = append(workflows, port.KeepWorkflow{
			ID:            w.ID,
			Name:          w.Name,
			WorkflowRawID: w.WorkflowRawID,
			Disabled:      w.Disabled,
		})
	}

	return workflows, nil
}

func (c *Client) CreateWorkflow(ctx context.Context, config port.WorkflowConfig) error {
	start := time.Now()
	reqURL := c.baseURL + "/workflows"

	// Keep API requires multipart/form-data with file field
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "workflow.yaml")
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := part.Write([]byte(config.Workflow)); err != nil {
		return fmt.Errorf("write workflow content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, &buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		duration := time.Since(start).Milliseconds()
		c.logger.Error("Keep CreateWorkflow failed",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", 0, duration, err.Error()),
		)
		keepCreateWorkflowErr.Inc()
		return fmt.Errorf("keep create workflow: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.logger.Error("Keep CreateWorkflow non-2xx",
			logger.ExternalFieldsWithError("keep", reqURL, "POST", resp.StatusCode, duration, string(respBody)),
		)
		keepCreateWorkflowErr.Inc()
		return fmt.Errorf("keep create workflow: status %d, body: %s", resp.StatusCode, respBody)
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	c.logger.Debug("Keep CreateWorkflow completed",
		logger.ExternalFields("keep", reqURL, "POST", resp.StatusCode, duration),
	)
	keepCreateWorkflowOK.Inc()

	return nil
}
