package usecase

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type mockKeepClient struct {
	enrichAlertErr        error
	enrichAlertCalled     bool
	enrichedEnrichments   map[string]string
	enrichedFingerprint   string
	unenrichAlertErr      error
	unenrichAlertCalled   bool
	unenrichFingerprint   string
	getAlertErr           error
	getAlertResponse      *port.KeepAlert
	providers             []port.KeepProvider
	workflows             []port.KeepWorkflow
	getProvidersErr       error
	createWebhookErr      error
	getWorkflowsErr       error
	createWorkflowErr     error
	createWebhookCalled   bool
	createWorkflowCalled  bool
	createdWebhookConfig  port.WebhookProviderConfig
	createdWorkflowConfig port.WorkflowConfig
}

func newMockKeepClient() *mockKeepClient {
	return &mockKeepClient{
		getAlertResponse: &port.KeepAlert{
			Fingerprint:     "fp-12345",
			Name:            "Test Alert",
			Status:          "firing",
			Severity:        "high",
			Description:     "Test description",
			Source:          []string{"prometheus"},
			Labels:          map[string]string{"env": "test"},
			FiringStartTime: time.Time{},
		},
	}
}

func (m *mockKeepClient) EnrichAlert(ctx context.Context, fingerprint string, enrichments map[string]string) error {
	m.enrichAlertCalled = true
	m.enrichedFingerprint = fingerprint
	m.enrichedEnrichments = enrichments
	if m.enrichAlertErr != nil {
		return m.enrichAlertErr
	}
	return nil
}

func (m *mockKeepClient) GetAlert(ctx context.Context, fingerprint string) (*port.KeepAlert, error) {
	if m.getAlertErr != nil {
		return nil, m.getAlertErr
	}
	resp := *m.getAlertResponse
	resp.Fingerprint = fingerprint
	return &resp, nil
}

func (m *mockKeepClient) UnenrichAlert(ctx context.Context, fingerprint string) error {
	m.unenrichAlertCalled = true
	m.unenrichFingerprint = fingerprint
	if m.unenrichAlertErr != nil {
		return m.unenrichAlertErr
	}
	return nil
}

func (m *mockKeepClient) GetProviders(ctx context.Context) ([]port.KeepProvider, error) {
	if m.getProvidersErr != nil {
		return nil, m.getProvidersErr
	}
	return m.providers, nil
}

func (m *mockKeepClient) CreateWebhookProvider(ctx context.Context, config port.WebhookProviderConfig) error {
	m.createWebhookCalled = true
	m.createdWebhookConfig = config
	if m.createWebhookErr != nil {
		return m.createWebhookErr
	}
	// Simulate provider being created - add to providers list
	m.providers = append(m.providers, port.KeepProvider{
		ID:   "created-provider-id",
		Type: "webhook",
		Name: config.Name,
	})
	return nil
}

func (m *mockKeepClient) GetWorkflows(ctx context.Context) ([]port.KeepWorkflow, error) {
	if m.getWorkflowsErr != nil {
		return nil, m.getWorkflowsErr
	}
	return m.workflows, nil
}

func (m *mockKeepClient) CreateWorkflow(ctx context.Context, config port.WorkflowConfig) error {
	m.createWorkflowCalled = true
	m.createdWorkflowConfig = config
	return m.createWorkflowErr
}

type mockUserMapper struct {
	mapping map[string]string
}

func newMockUserMapper() *mockUserMapper {
	return &mockUserMapper{
		mapping: make(map[string]string),
	}
}

func (m *mockUserMapper) GetKeepUsername(mattermostUsername string) string {
	if keepUser, ok := m.mapping[mattermostUsername]; ok {
		return keepUser
	}
	return ""
}

type mockMattermostClientCallback struct {
	getUserErr    error
	getUserFunc   func(ctx context.Context, userID string) (string, error)
	getUserCalled bool
}

func newMockMattermostClientCallback() *mockMattermostClientCallback {
	return &mockMattermostClientCallback{}
}

func (m *mockMattermostClientCallback) CreatePost(ctx context.Context, channelID string, attachment post.Attachment) (string, error) {
	return "post-123", nil
}

func (m *mockMattermostClientCallback) UpdatePost(ctx context.Context, postID string, attachment post.Attachment) error {
	return nil
}

func (m *mockMattermostClientCallback) GetUser(ctx context.Context, userID string) (string, error) {
	m.getUserCalled = true
	if m.getUserFunc != nil {
		return m.getUserFunc(ctx, userID)
	}
	if m.getUserErr != nil {
		return "", m.getUserErr
	}
	return "testuser", nil
}

type mockMessageBuilderCallback struct{}

func (m *mockMessageBuilderCallback) BuildFiringAttachment(a *alert.Alert, callbackURL, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#FF0000",
		Title: "FIRING: " + a.Name(),
	}
}

func (m *mockMessageBuilderCallback) BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment {
	return post.Attachment{
		Color: "#FFA500",
		Title: "ACKNOWLEDGED: " + a.Name(),
	}
}

func (m *mockMessageBuilderCallback) BuildResolvedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#00CC00",
		Title: "RESOLVED: " + a.Name(),
	}
}

func setupHandleCallbackUseCase() (*HandleCallbackUseCase, *mockPostRepository, *mockKeepClient, *mockMattermostClientCallback, *mockUserMapper) {
	postRepo := newMockPostRepository()
	keepClient := newMockKeepClient()
	mmClient := newMockMattermostClientCallback()
	msgBuilder := &mockMessageBuilderCallback{}
	userMapper := newMockUserMapper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewHandleCallbackUseCase(
		postRepo,
		keepClient,
		mmClient,
		msgBuilder,
		userMapper,
		"https://keep.example.com",
		"https://callback.example.com",
		logger,
	)

	return uc, postRepo, keepClient, mmClient, userMapper
}

func TestHandleCallbackUseCase_Acknowledge(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "acknowledged", keepClient.enrichedEnrichments["status"])
	assert.Contains(t, result.Ephemeral, "Alert acknowledged by @testuser")
	assert.Equal(t, "#FFA500", result.Attachment.Color)
	assert.Contains(t, result.Attachment.Title, "ACKNOWLEDGED")
}

func TestHandleCallbackUseCase_Resolve(t *testing.T) {
	uc, postRepo, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"))
	postRepo.posts[fp.Value()] = existingPost

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "resolved", keepClient.enrichedEnrichments["status"])
	assert.True(t, postRepo.deleteCalled)
	assert.Contains(t, result.Ephemeral, "Alert resolved by @testuser")
	assert.Equal(t, "#00CC00", result.Attachment.Color)
	assert.Contains(t, result.Attachment.Title, "RESOLVED")

	_, err = postRepo.FindByFingerprint(ctx, fp)
	assert.Equal(t, post.ErrNotFound, err)
}

func TestHandleCallbackUseCase_MissingFingerprint(t *testing.T) {
	uc, _, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action": "acknowledge",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleCallbackUseCase_EmptyFingerprint(t *testing.T) {
	uc, _, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleCallbackUseCase_InvalidSeverity(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	keepClient.getAlertResponse.Severity = "invalid"

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse severity")
}

func TestHandleCallbackUseCase_GetAlertError(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	keepClient.getAlertErr = errors.New("keep api error")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get alert from keep")
}

func TestHandleCallbackUseCase_EnrichAPIError(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	keepClient.enrichAlertErr = errors.New("keep api error")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Contains(t, result.Ephemeral, "Alert acknowledged by @testuser")
}

func TestHandleCallbackUseCase_PostNotFoundOnResolve(t *testing.T) {
	uc, postRepo, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.True(t, postRepo.deleteCalled)
	assert.Contains(t, result.Ephemeral, "Alert resolved by @testuser")
}

func TestHandleCallbackUseCase_UnknownAction(t *testing.T) {
	uc, _, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "unknown",
			"fingerprint": "fp-12345",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestHandleCallbackUseCase_GetUserError(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	mmClient.getUserErr = errors.New("user not found")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Contains(t, result.Ephemeral, "user-123")
}

func TestHandleCallbackUseCase_ResolveDeleteError(t *testing.T) {
	uc, postRepo, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	postRepo.deleteErr = errors.New("database error")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete post from store")
}

func TestHandleCallbackUseCase_AcknowledgeWithDifferentSeverities(t *testing.T) {
	severities := []string{"critical", "high", "warning", "info", "low"}

	for _, severity := range severities {
		t.Run("severity_"+severity, func(t *testing.T) {
			uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
			ctx := context.Background()

			keepClient.getAlertResponse.Severity = severity

			input := dto.MattermostCallbackInput{
				UserID: "user-123",
				Context: map[string]string{
					"action":      "acknowledge",
					"fingerprint": "fp-12345",
				},
			}

			result, err := uc.Execute(ctx, input)

			require.NoError(t, err)
			require.NotNil(t, result)
			time.Sleep(50 * time.Millisecond)
			assert.True(t, keepClient.enrichAlertCalled)
			assert.Contains(t, result.Ephemeral, "Alert acknowledged by @testuser")
		})
	}
}

func TestHandleCallbackUseCase_ResolveWithDifferentSeverities(t *testing.T) {
	severities := []string{"critical", "high", "warning", "info", "low"}

	for _, severity := range severities {
		t.Run("severity_"+severity, func(t *testing.T) {
			uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
			ctx := context.Background()

			keepClient.getAlertResponse.Severity = severity

			input := dto.MattermostCallbackInput{
				UserID: "user-123",
				Context: map[string]string{
					"action":      "resolve",
					"fingerprint": "fp-12345",
				},
			}

			result, err := uc.Execute(ctx, input)

			require.NoError(t, err)
			require.NotNil(t, result)
			time.Sleep(50 * time.Millisecond)
			assert.True(t, keepClient.enrichAlertCalled)
			assert.Contains(t, result.Ephemeral, "Alert resolved by @testuser")
		})
	}
}

func TestHandleCallbackUseCase_Wait(t *testing.T) {
	t.Run("wait completes after background goroutines finish", func(t *testing.T) {
		uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
		ctx := context.Background()

		input := dto.MattermostCallbackInput{
			UserID: "user-123",
			Context: map[string]string{
				"action":      "acknowledge",
				"fingerprint": "fp-12345",
			},
		}

		_, err := uc.Execute(ctx, input)
		require.NoError(t, err)

		uc.Wait()

		assert.True(t, keepClient.enrichAlertCalled)
	})

	t.Run("wait returns immediately when no goroutines pending", func(t *testing.T) {
		uc, _, _, _, _ := setupHandleCallbackUseCase()

		done := make(chan struct{})
		go func() {
			uc.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Wait did not return immediately when no goroutines pending")
		}
	})
}

func TestHandleCallbackUseCase_Unacknowledge(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "unacknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.unenrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.unenrichFingerprint)
	assert.Contains(t, result.Ephemeral, "Alert unacknowledged by @testuser")
	assert.Equal(t, "#FF0000", result.Attachment.Color)
	assert.Contains(t, result.Attachment.Title, "FIRING")
}

func TestHandleCallbackUseCase_UnacknowledgeAPIError(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	keepClient.unenrichAlertErr = errors.New("keep api error")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "unacknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.unenrichAlertCalled)
	assert.Contains(t, result.Ephemeral, "Alert unacknowledged by @testuser")
}

func TestHandleCallbackUseCase_UnacknowledgeWithDifferentSeverities(t *testing.T) {
	severities := []string{"critical", "high", "warning", "info", "low"}

	for _, severity := range severities {
		t.Run("severity_"+severity, func(t *testing.T) {
			uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
			ctx := context.Background()

			keepClient.getAlertResponse.Severity = severity

			input := dto.MattermostCallbackInput{
				UserID: "user-123",
				Context: map[string]string{
					"action":      "unacknowledge",
					"fingerprint": "fp-12345",
				},
			}

			result, err := uc.Execute(ctx, input)

			require.NoError(t, err)
			require.NotNil(t, result)
			time.Sleep(50 * time.Millisecond)
			assert.True(t, keepClient.unenrichAlertCalled)
			assert.Contains(t, result.Ephemeral, "Alert unacknowledged by @testuser")
		})
	}
}

func TestHandleCallbackUseCase_AcknowledgeWithUserMapping(t *testing.T) {
	uc, _, keepClient, _, userMapper := setupHandleCallbackUseCase()
	ctx := context.Background()

	userMapper.mapping["testuser"] = "keep-user"

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "acknowledged", keepClient.enrichedEnrichments["status"])
	assert.Equal(t, "keep-user", keepClient.enrichedEnrichments["assignee"])
}

func TestHandleCallbackUseCase_ResolveWithUserMapping(t *testing.T) {
	uc, _, keepClient, _, userMapper := setupHandleCallbackUseCase()
	ctx := context.Background()

	userMapper.mapping["testuser"] = "keep-user"

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "resolved", keepClient.enrichedEnrichments["status"])
	assert.Equal(t, "keep-user", keepClient.enrichedEnrichments["assignee"])
}

func TestHandleCallbackUseCase_AcknowledgeWithoutUserMapping(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "acknowledged", keepClient.enrichedEnrichments["status"])
	_, hasAssignee := keepClient.enrichedEnrichments["assignee"]
	assert.False(t, hasAssignee, "should not have assignee when no mapping exists")
}
