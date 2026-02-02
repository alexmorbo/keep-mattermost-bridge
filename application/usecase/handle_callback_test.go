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
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type mockKeepClient struct {
	enrichAlertErr      error
	enrichAlertCalled   bool
	enrichedStatus      string
	enrichedFingerprint string
}

func newMockKeepClient() *mockKeepClient {
	return &mockKeepClient{}
}

func (m *mockKeepClient) EnrichAlert(ctx context.Context, fingerprint, status string) error {
	m.enrichAlertCalled = true
	m.enrichedFingerprint = fingerprint
	m.enrichedStatus = status
	if m.enrichAlertErr != nil {
		return m.enrichAlertErr
	}
	return nil
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

func setupHandleCallbackUseCase() (*HandleCallbackUseCase, *mockPostRepository, *mockKeepClient, *mockMattermostClientCallback) {
	postRepo := newMockPostRepository()
	keepClient := newMockKeepClient()
	mmClient := newMockMattermostClientCallback()
	msgBuilder := &mockMessageBuilderCallback{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewHandleCallbackUseCase(
		postRepo,
		keepClient,
		mmClient,
		msgBuilder,
		"https://keep.example.com",
		"https://callback.example.com",
		logger,
	)

	return uc, postRepo, keepClient, mmClient
}

func TestHandleCallbackUseCase_Acknowledge(t *testing.T) {
	uc, _, keepClient, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "acknowledged", keepClient.enrichedStatus)
	assert.Contains(t, result.Ephemeral, "Alert acknowledged by @testuser")
	assert.Equal(t, "#FFA500", result.Attachment.Color)
	assert.Contains(t, result.Attachment.Title, "ACKNOWLEDGED")
}

func TestHandleCallbackUseCase_Resolve(t *testing.T) {
	uc, postRepo, keepClient, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"))
	postRepo.posts[fp.Value()] = existingPost

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
		},
	}

	result, err := uc.Execute(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, keepClient.enrichAlertCalled)
	assert.Equal(t, "fp-12345", keepClient.enrichedFingerprint)
	assert.Equal(t, "resolved", keepClient.enrichedStatus)
	assert.True(t, postRepo.deleteCalled)
	assert.Contains(t, result.Ephemeral, "Alert resolved by @testuser")
	assert.Equal(t, "#00CC00", result.Attachment.Color)
	assert.Contains(t, result.Attachment.Title, "RESOLVED")

	_, err = postRepo.FindByFingerprint(ctx, fp)
	assert.Equal(t, post.ErrNotFound, err)
}

func TestHandleCallbackUseCase_MissingFingerprint(t *testing.T) {
	uc, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":     "acknowledge",
			"alert_name": "Test Alert",
			"severity":   "high",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleCallbackUseCase_EmptyFingerprint(t *testing.T) {
	uc, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "",
			"alert_name":  "Test Alert",
			"severity":    "high",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleCallbackUseCase_InvalidSeverity(t *testing.T) {
	uc, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "invalid",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse severity")
}

func TestHandleCallbackUseCase_KeepAPIError(t *testing.T) {
	uc, _, keepClient, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	keepClient.enrichAlertErr = errors.New("keep api error")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
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
	uc, postRepo, keepClient, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
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
	uc, _, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "unknown",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}

func TestHandleCallbackUseCase_GetUserError(t *testing.T) {
	uc, _, keepClient, mmClient := setupHandleCallbackUseCase()
	ctx := context.Background()

	mmClient.getUserErr = errors.New("user not found")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "acknowledge",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
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
	uc, postRepo, _, _ := setupHandleCallbackUseCase()
	ctx := context.Background()

	postRepo.deleteErr = errors.New("database error")

	input := dto.MattermostCallbackInput{
		UserID: "user-123",
		Context: map[string]string{
			"action":      "resolve",
			"fingerprint": "fp-12345",
			"alert_name":  "Test Alert",
			"severity":    "high",
		},
	}

	_, err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete post from store")
}

func TestHandleCallbackUseCase_AcknowledgeWithDifferentSeverities(t *testing.T) {
	severities := []string{"critical", "high", "warning", "info"}

	for _, severity := range severities {
		t.Run("severity_"+severity, func(t *testing.T) {
			uc, _, keepClient, _ := setupHandleCallbackUseCase()
			ctx := context.Background()

			input := dto.MattermostCallbackInput{
				UserID: "user-123",
				Context: map[string]string{
					"action":      "acknowledge",
					"fingerprint": "fp-12345",
					"alert_name":  "Test Alert",
					"severity":    severity,
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
	severities := []string{"critical", "high", "warning", "info"}

	for _, severity := range severities {
		t.Run("severity_"+severity, func(t *testing.T) {
			uc, _, keepClient, _ := setupHandleCallbackUseCase()
			ctx := context.Background()

			input := dto.MattermostCallbackInput{
				UserID: "user-123",
				Context: map[string]string{
					"action":      "resolve",
					"fingerprint": "fp-12345",
					"alert_name":  "Test Alert",
					"severity":    severity,
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
