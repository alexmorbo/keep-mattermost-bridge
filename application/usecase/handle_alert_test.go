package usecase

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type mockPostRepository struct {
	posts        map[string]*post.Post
	findErr      error
	saveErr      error
	deleteErr    error
	saveCalled   bool
	deleteCalled bool
}

func newMockPostRepository() *mockPostRepository {
	return &mockPostRepository{
		posts: make(map[string]*post.Post),
	}
}

func (m *mockPostRepository) Save(ctx context.Context, fingerprint alert.Fingerprint, p *post.Post) error {
	m.saveCalled = true
	if m.saveErr != nil {
		return m.saveErr
	}
	m.posts[fingerprint.Value()] = p
	return nil
}

func (m *mockPostRepository) FindByFingerprint(ctx context.Context, fingerprint alert.Fingerprint) (*post.Post, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	p, ok := m.posts[fingerprint.Value()]
	if !ok {
		return nil, post.ErrNotFound
	}
	return p, nil
}

func (m *mockPostRepository) Delete(ctx context.Context, fingerprint alert.Fingerprint) error {
	m.deleteCalled = true
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.posts, fingerprint.Value())
	return nil
}

type mockMattermostClient struct {
	createPostErr    error
	updatePostErr    error
	createdPostID    string
	updatedPostID    string
	channelID        string
	createPostCalled bool
	updatePostCalled bool
}

func newMockMattermostClient() *mockMattermostClient {
	return &mockMattermostClient{
		createdPostID: "post-123",
		channelID:     "channel-456",
	}
}

func (m *mockMattermostClient) CreatePost(ctx context.Context, channelID string, attachment post.Attachment) (string, error) {
	m.createPostCalled = true
	if m.createPostErr != nil {
		return "", m.createPostErr
	}
	return m.createdPostID, nil
}

func (m *mockMattermostClient) UpdatePost(ctx context.Context, postID string, attachment post.Attachment) error {
	m.updatePostCalled = true
	m.updatedPostID = postID
	if m.updatePostErr != nil {
		return m.updatePostErr
	}
	return nil
}

func (m *mockMattermostClient) GetUser(ctx context.Context, userID string) (string, error) {
	return "testuser", nil
}

type mockMessageBuilder struct{}

func (m *mockMessageBuilder) BuildFiringAttachment(a *alert.Alert, callbackURL, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#FF0000",
		Title: "FIRING: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment {
	return post.Attachment{
		Color: "#FFA500",
		Title: "ACKNOWLEDGED: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildResolvedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#00CC00",
		Title: "RESOLVED: " + a.Name(),
	}
}

type mockChannelResolver struct {
	channel string
}

func newMockChannelResolver() *mockChannelResolver {
	return &mockChannelResolver{channel: "channel-456"}
}

func (m *mockChannelResolver) ChannelIDForSeverity(severity string) string {
	return m.channel
}

func setupHandleAlertUseCase() (*HandleAlertUseCase, *mockPostRepository, *mockMattermostClient) {
	postRepo := newMockPostRepository()
	mmClient := newMockMattermostClient()
	msgBuilder := &mockMessageBuilder{}
	channelResolver := newMockChannelResolver()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewHandleAlertUseCase(
		postRepo,
		mmClient,
		msgBuilder,
		channelResolver,
		"https://keep.example.com",
		"https://callback.example.com",
		logger,
	)

	return uc, postRepo, mmClient
}

func TestHandleAlertUseCase_NewFiringAlert(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{"env": "prod", "service": "api"}`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
	assert.False(t, mmClient.updatePostCalled)

	fp, _ := alert.NewFingerprint("fp-12345")
	savedPost, err := postRepo.FindByFingerprint(ctx, fp)
	require.NoError(t, err)
	assert.Equal(t, "post-123", savedPost.PostID())
	assert.Equal(t, "channel-456", savedPost.ChannelID())
	assert.Equal(t, "fp-12345", savedPost.Fingerprint().Value())
	assert.Equal(t, "Test Alert", savedPost.AlertName())
	assert.Equal(t, "high", savedPost.Severity().Value())
}

func TestHandleAlertUseCase_RefireExistingAlert(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"))
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.createPostCalled)
	assert.True(t, mmClient.updatePostCalled)
	assert.True(t, postRepo.saveCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)
}

func TestHandleAlertUseCase_ResolveExistingAlert(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"))
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)
	assert.True(t, postRepo.deleteCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)

	_, err = postRepo.FindByFingerprint(ctx, fp)
	assert.Equal(t, post.ErrNotFound, err)
}

func TestHandleAlertUseCase_ResolveWithoutExistingPost(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.updatePostCalled)
	assert.False(t, postRepo.deleteCalled)
}

func TestHandleAlertUseCase_InvalidFingerprint(t *testing.T) {
	uc, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleAlertUseCase_InvalidSeverity(t *testing.T) {
	uc, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "invalid",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse severity")
}

func TestHandleAlertUseCase_MattermostCreatePostError(t *testing.T) {
	uc, _, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	mmClient.createPostErr = errors.New("mattermost error")

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create mattermost post")
}

func TestHandleAlertUseCase_LabelParsingWithPythonDict(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "critical",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{'env': 'production', 'service': 'api', 'region': 'us-east-1'}`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_InvalidLabelsDoesNotFailAlert(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "warning",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `this is not valid`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_RepositorySaveError(t *testing.T) {
	uc, postRepo, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	postRepo.saveErr = errors.New("database error")

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save post to store")
}

func TestHandleAlertUseCase_RepositoryDeleteError(t *testing.T) {
	uc, postRepo, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"))
	postRepo.posts[fp.Value()] = existingPost
	postRepo.deleteErr = errors.New("database error")

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete post from store")
}

func TestHandleAlertUseCase_AcknowledgedStatusIsIgnored(t *testing.T) {
	uc, postRepo, mmClient := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "acknowledged",
		Description: "Test description",
		Source:      "prometheus",
		Labels:      `{}`,
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.createPostCalled)
	assert.False(t, mmClient.updatePostCalled)
	assert.False(t, postRepo.saveCalled)
	assert.False(t, postRepo.deleteCalled)
}
