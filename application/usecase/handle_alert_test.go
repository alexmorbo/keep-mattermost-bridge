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

func (m *mockPostRepository) FindAllActive(ctx context.Context) ([]*post.Post, error) {
	result := make([]*post.Post, 0, len(m.posts))
	for _, p := range m.posts {
		result = append(result, p)
	}
	return result, nil
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
	alerts      []*port.KeepAlert // different responses per call (for retry testing)
	getAlertErr error
	callCount   int
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
	m.callCount++
	if m.getAlertErr != nil {
		return nil, m.getAlertErr
	}
	// If alerts slice is set, return based on call count (for retry testing)
	if len(m.alerts) > 0 {
		idx := m.callCount - 1
		if idx >= len(m.alerts) {
			idx = len(m.alerts) - 1
		}
		return m.alerts[idx], nil
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

func (m *mockKeepClientForAlert) GetAlerts(ctx context.Context, limit int) ([]port.KeepAlert, error) {
	return nil, nil
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

func (m *mockMessageBuilder) BuildSuppressedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#9370DB",
		Title: "SUPPRESSED: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildPendingAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#87CEEB",
		Title: "PENDING: " + a.Name(),
	}
}

func (m *mockMessageBuilder) BuildMaintenanceAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{
		Color: "#708090",
		Title: "MAINTENANCE: " + a.Name(),
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

type mockUserMapperForAlert struct {
	mapping map[string]string
}

func newMockUserMapperForAlert() *mockUserMapperForAlert {
	return &mockUserMapperForAlert{
		mapping: make(map[string]string),
	}
}

func (m *mockUserMapperForAlert) GetKeepUsername(mattermostUsername string) (string, bool) {
	keepUser, ok := m.mapping[mattermostUsername]
	return keepUser, ok
}

func (m *mockUserMapperForAlert) GetMattermostUsername(keepUsername string) (string, bool) {
	for mmUser, keepUser := range m.mapping {
		if keepUser == keepUsername {
			return mmUser, true
		}
	}
	return "", false
}

func setupHandleAlertUseCase() (*HandleAlertUseCase, *mockPostRepository, *mockMattermostClient, *mockKeepClientForAlert, *mockMessageBuilder, *mockUserMapperForAlert) {
	postRepo := newMockPostRepository()
	mmClient := newMockMattermostClient()
	keepClient := newMockKeepClientForAlert()
	msgBuilder := &mockMessageBuilder{}
	channelResolver := newMockChannelResolver()
	userMapper := newMockUserMapperForAlert()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewHandleAlertUseCase(
		postRepo,
		mmClient,
		keepClient,
		msgBuilder,
		channelResolver,
		userMapper,
		"https://keep.example.com",
		"https://callback.example.com",
		logger,
	)

	return uc, postRepo, mmClient, keepClient, msgBuilder, userMapper
}

func TestHandleAlertUseCase_NewFiringAlert(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, _, _, _, _, _ := setupHandleAlertUseCase()
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
	uc, _, _, _, _, _ := setupHandleAlertUseCase()
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
	uc, _, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, _, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, _, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, keepClient, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, _, msgBuilder, _ := setupHandleAlertUseCase()
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
	uc, postRepo, mmClient, keepClient, msgBuilder, userMapper := setupHandleAlertUseCase()
	// Set up reverse mapping: Keep user "john.doe@keep" -> Mattermost user "john.doe"
	userMapper.mapping["john.doe"] = "john.doe@keep"
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "resolved",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "john.doe@keep"}, // Keep username in enrichment
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
	// Should use reverse-mapped Mattermost username, not Keep username
	assert.Contains(t, mmClient.lastReplyMessage, "john.doe")
	assert.Equal(t, "john.doe", msgBuilder.lastResolvedAssignee)
}

func TestHandleAlertUseCase_ResolveWithUnmappedAssigneeFallsBackToKeepUsername(t *testing.T) {
	uc, postRepo, mmClient, keepClient, msgBuilder, _ := setupHandleAlertUseCase()
	// userMapper has no mappings - should fallback to Keep username
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "resolved",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "unmapped@keep"},
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
	// Should use Keep username when no reverse mapping exists
	assert.Equal(t, "unmapped@keep", msgBuilder.lastResolvedAssignee)
}

func TestHandleAlertUseCase_RefireAcknowledgedAlertStaysAcknowledged(t *testing.T) {
	uc, postRepo, mmClient, keepClient, _, userMapper := setupHandleAlertUseCase()
	ctx := context.Background()
	// Set up reverse mapping: Keep user -> Mattermost user
	userMapper.mapping["john.doe"] = "john.doe@keep"

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "john.doe@keep", "status": "acknowledged"},
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

// Tests for fetchAssigneeWithRetry

func TestFetchAssigneeWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	uc, _, _, keepClient, msgBuilder, userMapper := setupHandleAlertUseCase()
	userMapper.mapping["john.doe"] = "john.doe@keep"
	ctx := context.Background()

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "john.doe@keep"},
	}

	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")

	assert.Equal(t, "john.doe", assignee)
	assert.Equal(t, 1, keepClient.callCount, "should only make 1 API call when assignee found immediately")
	_ = msgBuilder // silence unused
}

func TestFetchAssigneeWithRetry_SucceedsOnSecondAttempt(t *testing.T) {
	uc, _, _, keepClient, _, userMapper := setupHandleAlertUseCase()
	userMapper.mapping["john.doe"] = "john.doe@keep"
	ctx := context.Background()

	// First call: no assignee, second call: assignee present
	keepClient.alerts = []*port.KeepAlert{
		{
			Fingerprint: "fp-12345",
			Name:        "Test Alert",
			Status:      "acknowledged",
			Severity:    "high",
			Enrichments: nil, // no assignee on first call
		},
		{
			Fingerprint: "fp-12345",
			Name:        "Test Alert",
			Status:      "acknowledged",
			Severity:    "high",
			Enrichments: map[string]string{"assignee": "john.doe@keep"},
		},
	}

	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")

	assert.Equal(t, "john.doe", assignee)
	assert.Equal(t, 2, keepClient.callCount, "should make 2 API calls")
}

func TestFetchAssigneeWithRetry_SucceedsOnThirdAttempt(t *testing.T) {
	uc, _, _, keepClient, _, userMapper := setupHandleAlertUseCase()
	userMapper.mapping["john.doe"] = "john.doe@keep"
	ctx := context.Background()

	// First two calls: no assignee, third call: assignee present
	keepClient.alerts = []*port.KeepAlert{
		{Fingerprint: "fp-12345", Name: "Test Alert", Status: "acknowledged", Severity: "high", Enrichments: nil},
		{Fingerprint: "fp-12345", Name: "Test Alert", Status: "acknowledged", Severity: "high", Enrichments: nil},
		{Fingerprint: "fp-12345", Name: "Test Alert", Status: "acknowledged", Severity: "high", Enrichments: map[string]string{"assignee": "john.doe@keep"}},
	}

	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")

	assert.Equal(t, "john.doe", assignee)
	assert.Equal(t, 3, keepClient.callCount, "should make 3 API calls")
}

func TestFetchAssigneeWithRetry_ExhaustsRetriesReturnsEmpty(t *testing.T) {
	uc, _, _, keepClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	// All calls return no assignee
	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: nil, // no assignee
	}

	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")

	assert.Equal(t, "", assignee, "should return empty when assignee not found after all retries")
	assert.Equal(t, 4, keepClient.callCount, "should make 4 API calls (1 initial + 3 retries)")
}

func TestFetchAssigneeWithRetry_APIErrorAbortsRetry(t *testing.T) {
	uc, _, _, keepClient, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	keepClient.getAlertErr = errors.New("API error")

	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")

	assert.Equal(t, "", assignee, "should return empty on API error")
	assert.Equal(t, 1, keepClient.callCount, "should only make 1 API call when error occurs")
}

func TestFetchAssigneeWithRetry_RespectsContextCancellation(t *testing.T) {
	uc, _, _, keepClient, _, _ := setupHandleAlertUseCase()

	// All calls return no assignee - will trigger retries
	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: nil,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately - should abort during first retry wait
	cancel()

	start := time.Now()
	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")
	elapsed := time.Since(start)

	assert.Equal(t, "", assignee, "should return empty when context cancelled")
	// Should return quickly without waiting for all retries (total would be ~700ms)
	assert.Less(t, elapsed, 200*time.Millisecond, "should abort quickly when context is cancelled")
}

func TestFetchAssigneeWithRetry_FallsBackToKeepUsername(t *testing.T) {
	uc, _, _, keepClient, _, _ := setupHandleAlertUseCase()
	// No user mapping configured - should return Keep username as-is
	ctx := context.Background()

	keepClient.alert = &port.KeepAlert{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Status:      "acknowledged",
		Severity:    "high",
		Enrichments: map[string]string{"assignee": "unmapped@keep.local"},
	}

	assignee := uc.fetchAssigneeWithRetry(ctx, "fp-12345")

	assert.Equal(t, "unmapped@keep.local", assignee, "should return Keep username when no mapping exists")
}

// Tests for new statuses: suppressed, pending, maintenance

func TestHandleAlertUseCase_SuppressedStatusCreatesPost(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "suppressed",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_SuppressedStatusUpdatesExistingPost(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "suppressed",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.createPostCalled)
	assert.True(t, mmClient.updatePostCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)
}

func TestHandleAlertUseCase_PendingStatusCreatesPost(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "pending",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_PendingStatusUpdatesExistingPost(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "pending",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.createPostCalled)
	assert.True(t, mmClient.updatePostCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)
}

func TestHandleAlertUseCase_MaintenanceStatusCreatesPost(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "maintenance",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.True(t, mmClient.createPostCalled)
	assert.True(t, postRepo.saveCalled)
}

func TestHandleAlertUseCase_MaintenanceStatusUpdatesExistingPost(t *testing.T) {
	uc, postRepo, mmClient, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	fp, _ := alert.NewFingerprint("fp-12345")
	existingPost := post.NewPost("existing-post-123", "channel-456", alert.RestoreFingerprint("fp-12345"), "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = existingPost

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "maintenance",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.NoError(t, err)
	assert.False(t, mmClient.createPostCalled)
	assert.True(t, mmClient.updatePostCalled)
	assert.Equal(t, "existing-post-123", mmClient.updatedPostID)
}

func TestHandleAlertUseCase_InvalidStatusReturnsError(t *testing.T) {
	uc, _, _, _, _, _ := setupHandleAlertUseCase()
	ctx := context.Background()

	input := dto.KeepAlertInput{
		Fingerprint: "fp-12345",
		Name:        "Test Alert",
		Severity:    "high",
		Status:      "unknown_status",
		Description: "Test description",
		Source:      []string{"prometheus"},
		Labels:      map[string]string{},
	}

	err := uc.Execute(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse status")
}
