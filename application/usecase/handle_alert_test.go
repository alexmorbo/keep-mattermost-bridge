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
	createPostErr       error
	updatePostErr       error
	createdPostID       string
	updatedPostID       string
	channelID           string
	createPostCalled    bool
	updatePostCalled    bool
	replyToThreadCalled bool
	lastReplyMessage    string
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

func (m *mockMattermostClient) ReplyToThread(ctx context.Context, channelID, rootID, message string) error {
	m.replyToThreadCalled = true
	m.lastReplyMessage = message
	return nil
}

type mockKeepClientForAlert struct {
	alert       *port.KeepAlert
	getAlertErr error
}

func newMockKeepClientForAlert() *mockKeepClientForAlert {
	return &mockKeepClientForAlert{
		alert: &port.KeepAlert{
			Fingerprint: "fp-12345",
			Name:        "Test Alert",
			Status:      "firing",
			Severity:    "high",
			Enrichments: nil,
		},
	}
}

func (m *mockKeepClientForAlert) EnrichAlert(ctx context.Context, fingerprint string, enrichments map[string]string, opts port.EnrichOptions) error {
	return nil
}

func (m *mockKeepClientForAlert) UnenrichAlert(ctx context.Context, fingerprint string, enrichments []string) error {
	return nil
}

func (m *mockKeepClientForAlert) GetAlert(ctx context.Context, fingerprint string) (*port.KeepAlert, error) {
	if m.getAlertErr != nil {
		return nil, m.getAlertErr
	}
	return m.alert, nil
}

func (m *mockKeepClientForAlert) GetProviders(ctx context.Context) ([]port.KeepProvider, error) {
	return nil, nil
}

func (m *mockKeepClientForAlert) CreateWebhookProvider(ctx context.Context, config port.WebhookProviderConfig) error {
	return nil
}

func (m *mockKeepClientForAlert) GetWorkflows(ctx context.Context) ([]port.KeepWorkflow, error) {
	return nil, nil
}

func (m *mockKeepClientForAlert) CreateWorkflow(ctx context.Context, config port.WorkflowConfig) error {
	return nil
}

type mockMessageBuilder struct {
	lastResolvedAlert        *alert.Alert
	lastResolvedAssignee     string
	lastAcknowledgedAssignee string
}

func (m *mockMessageBuilder) BuildFiringAttachment(a *alert.Alert, callbackURL, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#FF0000",
		Title: "FIRING: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment {
	m.lastAcknowledgedAssignee = username
	return post.Attachment{
		Color: "#FFA500",
		Title: "ACKNOWLEDGED: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildResolvedAttachment(a *alert.Alert, keepUIURL, acknowledgedBy string) post.Attachment {
	m.lastResolvedAlert = a
	m.lastResolvedAssignee = acknowledgedBy
	return post.Attachment{
		Color: "#00CC00",
		Title: "RESOLVED: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildProcessingAttachment(attachmentJSON, action string) (post.Attachment, error) {
	return post.Attachment{
		Color: "#808080",
		Title: "Processing",
	}, nil
}

func (m *mockMessageBuilder) BuildErrorAttachment(alertName, fingerprint, keepUIURL, errorMsg string) post.Attachment {
	return post.Attachment{
		Color: "#FF0000",
		Title: alertName,
		Text:  "Error: " + errorMsg,
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

func setupHandleAlertUseCase() (*HandleAlertUseCase, *mockPostRepository, *mockMattermostClient, *mockKeepClientForAlert, *mockMessageBuilder) {
	postRepo := newMockPostRepository()
	mmClient := newMockMattermostClient()
	keepClient := newMockKeepClientForAlert()
	msgBuilder := &mockMessageBuilder{}
	channelResolver := newMockChannelResolver()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewHandleAlertUseCase(
		postRepo,
		mmClient,
		keepClient,
		msgBuilder,
		channelResolver,
		"https://keep.example.com",
		"https://callback.example.com",
		logger,
	)

	return uc, postRepo, mmClient, keepClient, msgBuilder
}

func TestHandleAlertUseCase_NewFiringAlert(t *testing.T) {
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{"env": "prod", "service": "api"},
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
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.createPostCalled)
	assert.True(t, mmClient.updatePostCalled)
	assert.True(t, postRepo.saveCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)
}

func TestHandleAlertUseCase_ResolveExistingAlert(t *testing.T) {
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
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
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.updatePostCalled)
	assert.False(t, postRepo.deleteCalled)
}

func TestHandleAlertUseCase_InvalidFingerprint(t *testing.T) {
	uc, _, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fingerprint")
}

func TestHandleAlertUseCase_InvalidSeverity(t *testing.T) {
	uc, _, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "invalid",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse severity")
}

func TestHandleAlertUseCase_MattermostCreatePostError(t *testing.T) {
	uc, _, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	mmClient.createPostErr = errors.New("mattermost error")

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create mattermost post")
}

func TestHandleAlertUseCase_LabelParsingWithPythonDict(t *testing.T) {
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "critical",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{"env": "production", "service": "api", "region": "us-east-1"},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_InvalidLabelsDoesNotFailAlert(t *testing.T) {
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "warning",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_RepositorySaveError(t *testing.T) {
	uc, postRepo, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	postRepo.saveErr = errors.New("database error")

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save post to store")
}

func TestHandleAlertUseCase_RepositoryDeleteError(t *testing.T) {
	uc, postRepo, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost
	postRepo.deleteErr = errors.New("database error")

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete post from store")
}

func TestHandleAlertUseCase_AcknowledgedStatusUpdatesPost(t *testing.T) {
	uc, postRepo, mmClient, keepClient, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "john.doe"},
	}

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "acknowledged",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)
}

func TestHandleAlertUseCase_AcknowledgedWithoutExistingPostCreatesPost(t *testing.T) {
	uc, postRepo, mmClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "acknowledged",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_ResolveUsesStoredFiringStartTime(t *testing.T) {
	uc, postRepo, mmClient, _, msgBuilder := setupHandleAlertUseCase()
	ctx := context.Background()

	storedFiringTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), storedFiringTime)
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint:     "fp-12345",
		Name:            "Test Alert",
		Severity:        "high",
		Status:          "resolved",
		Description:     "Test description",
		Source:          []string{"prometheus"},
		Labels:          map[string]string{},
		FiringStartTime: "",
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)

	require.NotNil(t, msgBuilder.lastResolvedAlert, "BuildResolvedAttachment should have been called")
	assert.Equal(t, storedFiringTime, msgBuilder.lastResolvedAlert.FiringStartTime(),
		"resolved alert should use firingStartTime from stored post, not from incoming alert")
}

func TestHandleAlertUseCase_ResolveWithAssigneeShowsInFooter(t *testing.T) {
	uc, postRepo, mmClient, keepClient, msgBuilder := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "resolved",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "john.doe"},
	}

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "resolved",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)
	assert.True(t, mmClient.replyToThreadCalled)
	assert.Contains(t, mmClient.lastReplyMessage, "john.doe")
	assert.Equal(t, "john.doe", msgBuilder.lastResolvedAssignee)
}

func TestHandleAlertUseCase_RefireAcknowledgedAlertStaysAcknowledged(t *testing.T) {
	uc, postRepo, mmClient, keepClient, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "john.doe", "status": "acknowledged"},
	}

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "firing",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)
	assert.True(t, mmClient.replyToThreadCalled)
	assert.Contains(t, mmClient.lastReplyMessage, "re-fired")
	assert.Contains(t, mmClient.lastReplyMessage, "john.doe")
}
