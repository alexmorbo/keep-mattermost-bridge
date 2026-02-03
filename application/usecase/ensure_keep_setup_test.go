package usecase

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
)

func setupEnsureKeepSetupUseCase() (*EnsureKeepSetupUseCase, *mockKeepClient) {
	keepClient := newMockKeepClient()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewEnsureKeepSetupUseCase(
		keepClient,
		"https://kmbridge.example.com/webhook",
		logger,
	)

	return uc, keepClient
}

func TestEnsureKeepSetupUseCase_CreatesProviderAndWorkflow(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, keepClient.createWebhookCalled)
	assert.True(t, keepClient.createWorkflowCalled)

	assert.Equal(t, "kmbridge", keepClient.createdWebhookConfig.Name)
	assert.Equal(t, "https://kmbridge.example.com/webhook", keepClient.createdWebhookConfig.URL)
	assert.Equal(t, "POST", keepClient.createdWebhookConfig.Method)
	assert.False(t, keepClient.createdWebhookConfig.Verify)

	assert.Equal(t, "kmbridge-webhook", keepClient.createdWorkflowConfig.ID)
	assert.Equal(t, "Mattermost updates via kmbridge", keepClient.createdWorkflowConfig.Name)
	assert.Contains(t, keepClient.createdWorkflowConfig.Workflow, "providers.kmbridge")
}

func TestEnsureKeepSetupUseCase_ProviderAlreadyExists(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.providers = []port.KeepProvider{
		{ID: "provider-123", Type: "webhook", Name: "kmbridge"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.False(t, keepClient.createWebhookCalled)
	assert.True(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_WorkflowAlreadyExists(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.workflows = []port.KeepWorkflow{
		{ID: "workflow-123", Name: "Mattermost updates via kmbridge", WorkflowRawID: "kmbridge-webhook"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, keepClient.createWebhookCalled)
	assert.False(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_BothAlreadyExist(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.providers = []port.KeepProvider{
		{ID: "provider-123", Type: "webhook", Name: "kmbridge"},
	}
	keepClient.workflows = []port.KeepWorkflow{
		{ID: "workflow-123", Name: "Mattermost updates via kmbridge", WorkflowRawID: "kmbridge-webhook"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.False(t, keepClient.createWebhookCalled)
	assert.False(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_GetProvidersError(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.getProvidersErr = errors.New("api error")

	err := uc.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure provider")
	assert.Contains(t, err.Error(), "get providers")
	assert.False(t, keepClient.createWebhookCalled)
	assert.False(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_CreateProviderError(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.createWebhookErr = errors.New("create error")

	err := uc.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure provider")
	assert.Contains(t, err.Error(), "create webhook provider")
	assert.True(t, keepClient.createWebhookCalled)
	assert.False(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_GetWorkflowsError(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.getWorkflowsErr = errors.New("api error")

	err := uc.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure workflow")
	assert.Contains(t, err.Error(), "get workflows")
	assert.True(t, keepClient.createWebhookCalled)
	assert.False(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_CreateWorkflowError(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.createWorkflowErr = errors.New("create error")

	err := uc.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure workflow")
	assert.Contains(t, err.Error(), "create workflow")
	assert.True(t, keepClient.createWebhookCalled)
	assert.True(t, keepClient.createWorkflowCalled)
}

func TestEnsureKeepSetupUseCase_DifferentProviderTypesIgnored(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.providers = []port.KeepProvider{
		{ID: "provider-1", Type: "slack", Name: "kmbridge"},
		{ID: "provider-2", Type: "webhook", Name: "other-provider"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, keepClient.createWebhookCalled)
}

func TestEnsureKeepSetupUseCase_DifferentWorkflowIDsIgnored(t *testing.T) {
	uc, keepClient := setupEnsureKeepSetupUseCase()
	ctx := context.Background()

	keepClient.workflows = []port.KeepWorkflow{
		{ID: "workflow-1", Name: "Other workflow", WorkflowRawID: "other-workflow"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, keepClient.createWorkflowCalled)
}
