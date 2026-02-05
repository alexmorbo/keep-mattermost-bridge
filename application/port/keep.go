package port

import (
	"context"
	"time"
)

type KeepAlert struct {
	Fingerprint     string
	Name            string
	Status          string
	Severity        string
	Description     string
	Source          []string
	Labels          map[string]string
	FiringStartTime time.Time
}

type KeepProvider struct {
	ID      string
	Type    string
	Name    string
	Details map[string]any
}

type KeepWorkflow struct {
	ID            string
	Name          string
	WorkflowRawID string
	Disabled      bool
}

type WebhookProviderConfig struct {
	Name   string
	URL    string
	Method string
	Verify bool
}

type WorkflowConfig struct {
	ID          string
	Name        string
	Description string
	Workflow    string
}

type KeepClient interface {
	EnrichAlert(ctx context.Context, fingerprint string, enrichments map[string]string) error
	UnenrichAlert(ctx context.Context, fingerprint string, enrichments []string) error
	GetAlert(ctx context.Context, fingerprint string) (*KeepAlert, error)
	GetProviders(ctx context.Context) ([]KeepProvider, error)
	CreateWebhookProvider(ctx context.Context, config WebhookProviderConfig) error
	GetWorkflows(ctx context.Context) ([]KeepWorkflow, error)
	CreateWorkflow(ctx context.Context, config WorkflowConfig) error
}
