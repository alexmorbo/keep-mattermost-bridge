package usecase

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
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
	mu                    sync.Mutex
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enrichAlertCalled = true
	m.enrichedFingerprint = fingerprint
	m.enrichedEnrichments = enrichments
	if m.enrichAlertErr != nil {
		return m.enrichAlertErr
	}
	return nil
}

func (m *mockKeepClient) wasEnrichAlertCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enrichAlertCalled
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unenrichAlertCalled = true
	m.unenrichFingerprint = fingerprint
	if m.unenrichAlertErr != nil {
		return m.unenrichAlertErr
	}
	return nil
}

func (m *mockKeepClient) wasUnenrichAlertCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unenrichAlertCalled
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

func (m *mockUserMapper) GetKeepUsername(mattermostUsername string) (string, bool) {
	keepUser, ok := m.mapping[mattermostUsername]
	return keepUser, ok
}

type mockMattermostClientCallback struct {
	getUserErr         error
	getUserFunc        func(ctx context.Context, userID string) (string, error)
	getUserCalled      bool
	updatePostCalled   bool
	updatePostErr      error
	replyToThreadErr   error
	replyToThreadCalls []string
	mu                 sync.Mutex
}

func newMockMattermostClientCallback() *mockMattermostClientCallback {
	return &mockMattermostClientCallback{
		replyToThreadCalls: make([]string, 0),
	}
}

func (m *mockMattermostClientCallback) CreatePost(ctx context.Context, channelID string, attachment post.Attachment) (string, error) {
	return "post-123", nil
}

func (m *mockMattermostClientCallback) UpdatePost(ctx context.Context, postID string, attachment post.Attachment) error {
	m.mu.Lock()
	m.updatePostCalled = true
	m.mu.Unlock()
	return m.updatePostErr
}

func (m *mockMattermostClientCallback) wasUpdatePostCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updatePostCalled
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

func (m *mockMattermostClientCallback) ReplyToThread(ctx context.Context, channelID, rootID, message string) error {
	m.mu.Lock()
	m.replyToThreadCalls = append(m.replyToThreadCalls, message)
	m.mu.Unlock()
	return m.replyToThreadErr
}

func (m *mockMattermostClientCallback) getReplyToThreadCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.replyToThreadCalls
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

func (m *mockMessageBuilderCallback) BuildLoadingAttachment(action, alertName, fingerprint, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#808080",
		Title: alertName,
	}
}

func (m *mockMessageBuilderCallback) BuildErrorAttachment(alertName, fingerprint, keepUIURL, errorMsg string) post.Attachment {
	return post.Attachment{
		Color: "#FF0000",
		Title: alertName,
		Text:  "Error: " + errorMsg,
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

func TestHandleCallbackUseCase_ExecuteImmediate_ReturnsLoadingState(t *testing.T) {
	uc, _, _, _, _ := setupHandleCallbackUseCase()

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	result, err := uc.ExecuteImmediate(input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "#808080", result.Attachment.Color)
	assert.Equal(t, "Test Alert", result.Attachment.Title)
	assert.Empty(t, result.Ephemeral)
}

func TestHandleCallbackUseCase_ExecuteImmediate_MissingFingerprint(t *testing.T) {
	uc, _, _, _, _ := setupHandleCallbackUseCase()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action": "acknowledge",
		},
	}

	_, err := uc.ExecuteImmediate(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleCallbackUseCase_ExecuteAsync_Acknowledge(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()
	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "acknowledged", keepClient.enrichedEnrichments["status"])
	assert.True(t, mmClient.wasUpdatePostCalled())
	replies := mmClient.getReplyToThreadCalls()
	require.Len(t, replies, 1)
	assert.Contains(t, replies[0], "Acknowledged by @testuser")
}

func TestHandleCallbackUseCase_ExecuteAsync_Resolve(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"))
	postRepo.posts[fp.Value()] = existingPost

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.Equal(t, "resolved", keepClient.enrichedEnrichments["status"])
	assert.True(t, postRepo.deleteCalled)
	assert.True(t, mmClient.wasUpdatePostCalled())
	replies := mmClient.getReplyToThreadCalls()
	require.Len(t, replies, 1)
	assert.Contains(t, replies[0], "Resolved by @testuser")
}

func TestHandleCallbackUseCase_ExecuteAsync_Unacknowledge(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()
	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "unacknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasUnenrichAlertCalled())
	assert.Equal(t, "fp-12345", keepClient.unenrichFingerprint)
	assert.True(t, mmClient.wasUpdatePostCalled())
	replies := mmClient.getReplyToThreadCalls()
	require.Len(t, replies, 1)
	assert.Contains(t, replies[0], "Unacknowledged by @testuser")
}

func TestHandleCallbackUseCase_ExecuteAsync_GetAlertError(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	keepClient.getAlertErr = errors.New("keep api error")

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)

	assert.False(t, keepClient.wasEnrichAlertCalled())
	assert.False(t, mmClient.wasUpdatePostCalled())
}

func TestHandleCallbackUseCase_ExecuteAsync_InvalidSeverity(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	keepClient.getAlertResponse.Severity = "invalid"

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)

	assert.False(t, keepClient.wasEnrichAlertCalled())
	assert.False(t, mmClient.wasUpdatePostCalled())
}

func TestHandleCallbackUseCase_ExecuteAsync_GetUserError(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	mmClient.getUserErr = errors.New("user not found")

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	replies := mmClient.getReplyToThreadCalls()
	require.Len(t, replies, 1)
	assert.Contains(t, replies[0], "user-123")
}

func TestHandleCallbackUseCase_ExecuteAsync_EnrichAPIError(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	keepClient.enrichAlertErr = errors.New("keep api error")

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.True(t, mmClient.wasUpdatePostCalled())
}

func TestHandleCallbackUseCase_ExecuteAsync_UnknownAction(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()
	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "unknown",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)

	assert.False(t, keepClient.wasEnrichAlertCalled())
	assert.False(t, mmClient.wasUpdatePostCalled())
}

func TestHandleCallbackUseCase_ExecuteAsync_AcknowledgeWithUserMapping(t *testing.T) {
	uc, _, keepClient, _, userMapper := setupHandleCallbackUseCase()

	userMapper.mapping["testuser"] = "keep-user"

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.Equal(t, "acknowledged", keepClient.enrichedEnrichments["status"])
	assert.Equal(t, "keep-user", keepClient.enrichedEnrichments["assignee"])
}

func TestHandleCallbackUseCase_ExecuteAsync_AcknowledgeWithoutUserMapping(t *testing.T) {
	uc, _, keepClient, _, _ := setupHandleCallbackUseCase()
	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.Equal(t, "acknowledged", keepClient.enrichedEnrichments["status"])
	_, hasAssignee := keepClient.enrichedEnrichments["assignee"]
	assert.False(t, hasAssignee, "should not have assignee when no mapping exists")
}

func TestHandleCallbackUseCase_Wait(t *testing.T) {
	t.Run("wait completes after background goroutines finish", func(t *testing.T) {
		uc, _, keepClient, _, _ := setupHandleCallbackUseCase()

		input := dto.MattermostCallbackInput{
			UserID:    "user-123",
			PostID:    "post-456",
			ChannelID: "channel-789",
			Context: map[string]string{
				"action":      "acknowledge",
				"fingerprint": "fp-12345",
				"alert_name":  "Test Alert",
			},
		}

		uc.ExecuteAsync(input)
		uc.Wait()

		assert.True(t, keepClient.wasEnrichAlertCalled())
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

func TestHandleCallbackUseCase_ExecuteAsync_DifferentSeverities(t *testing.T) {
	severities := []string{"critical", "high", "warning", "info", "low"}

	for _, severity := range severities {
		t.Run("severity_"+severity, func(t *testing.T) {
			uc, _, keepClient, _, _ := setupHandleCallbackUseCase()

			keepClient.getAlertResponse.Severity = severity

			input := dto.MattermostCallbackInput{
				UserID:    "user-123",
				PostID:    "post-456",
				ChannelID: "channel-789",
				Context: map[string]string{
					"action":      "acknowledge",
					"fingerprint": "fp-12345",
					"alert_name":  "Test Alert",
				},
			}

			uc.ExecuteAsync(input)
			uc.Wait()

			assert.True(t, keepClient.wasEnrichAlertCalled())
		})
	}
}

func TestHandleCallbackUseCase_ExecuteAsync_UpdatePostError(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	mmClient.updatePostErr = errors.New("update post error")

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.True(t, mmClient.wasUpdatePostCalled())
	replies := mmClient.getReplyToThreadCalls()
	assert.Len(t, replies, 1, "should still attempt to reply to thread even if update fails")
}

func TestHandleCallbackUseCase_ExecuteAsync_ReplyToThreadError(t *testing.T) {
	uc, _, keepClient, mmClient, _ := setupHandleCallbackUseCase()

	mmClient.replyToThreadErr = errors.New("reply to thread error")

	input := dto.MattermostCallbackInput{
		UserID:    "user-123",
		PostID:    "post-456",
		ChannelID: "channel-789",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
		},
	}

	uc.ExecuteAsync(input)
	uc.Wait()

	assert.True(t, keepClient.wasEnrichAlertCalled())
	assert.True(t, mmClient.wasUpdatePostCalled())
	replies := mmClient.getReplyToThreadCalls()
	assert.Len(t, replies, 1, "should attempt to reply even if it will fail")
}

func TestHandleCallbackUseCase_ExecuteImmediate_MissingAlertName(t *testing.T) {
	uc, _, _, _, _ := setupHandleCallbackUseCase()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
		},
	}

	_, err := uc.ExecuteImmediate(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "alert_name")
}
