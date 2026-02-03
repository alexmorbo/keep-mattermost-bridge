package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

const (
	providerName  = "kmbridge"
	workflowRawID = "kmbridge-webhook"
)

type EnsureKeepSetupUseCase struct {
	keepClient port.KeepClient
	webhookURL string
	logger     *slog.Logger
}

func NewEnsureKeepSetupUseCase(
	keepClient port.KeepClient,
	webhookURL string,
	logger *slog.Logger,
) *EnsureKeepSetupUseCase {
	return &EnsureKeepSetupUseCase{
		keepClient: keepClient,
		webhookURL: webhookURL,
		logger:     logger,
	}
}

func (uc *EnsureKeepSetupUseCase) Execute(ctx context.Context) error {
	if err := uc.ensureProvider(ctx); err != nil {
		return fmt.Errorf("ensure provider: %w", err)
	}

	if err := uc.ensureWorkflow(ctx); err != nil {
		return fmt.Errorf("ensure workflow: %w", err)
	}

	return nil
}

func (uc *EnsureKeepSetupUseCase) ensureProvider(ctx context.Context) error {
	providers, err := uc.keepClient.GetProviders(ctx)
	if err != nil {
		return fmt.Errorf("get providers: %w", err)
	}

	for _, p := range providers {
		if p.Type == "webhook" && p.Name == providerName {
			uc.logger.Info("Keep webhook provider already exists",
				logger.ApplicationFields("provider_exists",
					slog.String("provider_name", providerName),
					slog.String("provider_id", p.ID),
				),
			)
			return nil
		}
	}

	uc.logger.Info("Creating Keep webhook provider",
		logger.ApplicationFields("provider_create",
			slog.String("provider_name", providerName),
			slog.String("webhook_url", uc.webhookURL),
		),
	)

	config := port.WebhookProviderConfig{
		Name:   providerName,
		URL:    uc.webhookURL,
		Method: "POST",
		Verify: false,
	}

	if err := uc.keepClient.CreateWebhookProvider(ctx, config); err != nil {
		return fmt.Errorf("create webhook provider: %w", err)
	}

	// Verify provider was created by checking it appears in the list
	providers, err = uc.keepClient.GetProviders(ctx)
	if err != nil {
		return fmt.Errorf("verify provider creation: %w", err)
	}

	for _, p := range providers {
		if p.Type == "webhook" && p.Name == providerName {
			uc.logger.Info("Keep webhook provider created and verified",
				logger.ApplicationFields("provider_created",
					slog.String("provider_name", providerName),
					slog.String("provider_id", p.ID),
				),
			)
			return nil
		}
	}

	return fmt.Errorf("provider created but not found in providers list")
}

func (uc *EnsureKeepSetupUseCase) ensureWorkflow(ctx context.Context) error {
	workflows, err := uc.keepClient.GetWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("get workflows: %w", err)
	}

	for _, w := range workflows {
		if w.WorkflowRawID == workflowRawID {
			uc.logger.Info("Keep workflow already exists",
				logger.ApplicationFields("workflow_exists",
					slog.String("workflow_raw_id", workflowRawID),
					slog.String("workflow_id", w.ID),
				),
			)
			return nil
		}
	}

	uc.logger.Info("Creating Keep workflow",
		logger.ApplicationFields("workflow_create",
			slog.String("workflow_raw_id", workflowRawID),
		),
	)

	workflowYAML := `id: kmbridge-webhook
description: Route alerts to Mattermost channels via kmbridge
disabled: false
triggers:
- type: alert
name: Mattermost updates via kmbridge
inputs: []
consts: {}
owners: []
services: []
steps: []
actions:
- name: webhook-action
  provider:
    type: webhook
    config: "{{ providers.kmbridge }}"
    with:
      body:
        id: "{{ alert.id }}"
        name: "{{ alert.name }}"
        status: "{{ alert.status }}"
        severity: "{{ alert.severity }}"
        source: "{{ alert.source }}"
        fingerprint: "{{ alert.fingerprint }}"
        description: "{{ alert.description }}"
        labels: "{{ alert.labels }}"
        firingStartTime: "{{ alert.firingStartTime }}"
  vars: {}`

	config := port.WorkflowConfig{
		ID:          workflowRawID,
		Name:        "Mattermost updates via kmbridge",
		Description: "Route alerts to Mattermost channels via kmbridge",
		Workflow:    workflowYAML,
	}

	if err := uc.keepClient.CreateWorkflow(ctx, config); err != nil {
		return fmt.Errorf("create workflow: %w", err)
	}

	uc.logger.Info("Keep workflow created successfully",
		logger.ApplicationFields("workflow_created",
			slog.String("workflow_raw_id", workflowRawID),
		),
	)

	return nil
}
