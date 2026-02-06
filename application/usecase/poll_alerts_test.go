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

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type mockPollPostRepository struct {
	posts      map[string]*post.Post
	findAllErr error
	saveErr    error
	saveCalled bool
}

func newMockPollPostRepository() *mockPollPostRepository {
	return &mockPollPostRepository{
		posts: make(map[string]*post.Post),
	}
}

func (m *mockPollPostRepository) Save(ctx context.Context, fingerprint alert.Fingerprint, p *post.Post) error {
	m.saveCalled = true
	if m.saveErr != nil {
		return m.saveErr
	}
	m.posts[fingerprint.Value()] = p
	return nil
}

func (m *mockPollPostRepository) FindByFingerprint(ctx context.Context, fingerprint alert.Fingerprint) (*post.Post, error) {
	p, ok := m.posts[fingerprint.Value()]
	if !ok {
		return nil, post.ErrNotFound
	}
	return p, nil
}

func (m *mockPollPostRepository) Delete(ctx context.Context, fingerprint alert.Fingerprint) error {
	delete(m.posts, fingerprint.Value())
	return nil
}

func (m *mockPollPostRepository) FindAllActive(ctx context.Context) ([]*post.Post, error) {
	if m.findAllErr != nil {
		return nil, m.findAllErr
	}
	result := make([]*post.Post, 0, len(m.posts))
	for _, p := range m.posts {
		result = append(result, p)
	}
	return result, nil
}

type mockPollKeepClient struct {
	alerts       []port.KeepAlert
	getAlertsErr error
}

func (m *mockPollKeepClient) EnrichAlert(ctx context.Context, fingerprint string, enrichments map[string]string, opts port.EnrichOptions) error {
	return nil
}

func (m *mockPollKeepClient) UnenrichAlert(ctx context.Context, fingerprint string, enrichments []string) error {
	return nil
}

func (m *mockPollKeepClient) GetAlert(ctx context.Context, fingerprint string) (*port.KeepAlert, error) {
	for _, a := range m.alerts {
		if a.Fingerprint == fingerprint {
			return &a, nil
		}
	}
	return nil, errors.New("alert not found")
}

func (m *mockPollKeepClient) GetAlerts(ctx context.Context, limit int) ([]port.KeepAlert, error) {
	if m.getAlertsErr != nil {
		return nil, m.getAlertsErr
	}
	return m.alerts, nil
}

func (m *mockPollKeepClient) GetProviders(ctx context.Context) ([]port.KeepProvider, error) {
	return nil, nil
}

func (m *mockPollKeepClient) CreateWebhookProvider(ctx context.Context, config port.WebhookProviderConfig) error {
	return nil
}

func (m *mockPollKeepClient) GetWorkflows(ctx context.Context) ([]port.KeepWorkflow, error) {
	return nil, nil
}

func (m *mockPollKeepClient) CreateWorkflow(ctx context.Context, config port.WorkflowConfig) error {
	return nil
}

type mockPollMattermostClient struct {
	updatePostCalled bool
	updatePostErr    error
	replyMessage     string
	replyErr         error
}

func (m *mockPollMattermostClient) CreatePost(ctx context.Context, channelID string, attachment post.Attachment) (string, error) {
	return "post-123", nil
}

func (m *mockPollMattermostClient) UpdatePost(ctx context.Context, postID string, attachment post.Attachment) error {
	m.updatePostCalled = true
	return m.updatePostErr
}

func (m *mockPollMattermostClient) GetUser(ctx context.Context, userID string) (string, error) {
	return "testuser", nil
}

func (m *mockPollMattermostClient) ReplyToThread(ctx context.Context, channelID, rootID, message string) error {
	m.replyMessage = message
	return m.replyErr
}

type mockPollMessageBuilder struct{}

func (m *mockPollMessageBuilder) BuildFiringAttachment(a *alert.Alert, callbackURL, keepUIURL string) post.Attachment {
	return post.Attachment{Color: "#FF0000", Title: "FIRING: " + a.Name()}
}

func (m *mockPollMessageBuilder) BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment {
	return post.Attachment{Color: "#FFA500", Title: "ACK: " + a.Name(), Footer: "Ack by " + username}
}

func (m *mockPollMessageBuilder) BuildResolvedAttachment(a *alert.Alert, keepUIURL, acknowledgedBy string) post.Attachment {
	return post.Attachment{Color: "#00FF00", Title: "RESOLVED: " + a.Name()}
}

func (m *mockPollMessageBuilder) BuildSuppressedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{}
}

func (m *mockPollMessageBuilder) BuildPendingAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{}
}

func (m *mockPollMessageBuilder) BuildMaintenanceAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	return post.Attachment{}
}

func (m *mockPollMessageBuilder) BuildProcessingAttachment(attachmentJSON, action string) (post.Attachment, error) {
	return post.Attachment{}, nil
}

func (m *mockPollMessageBuilder) BuildErrorAttachment(alertName, fingerprint, keepUIURL, errorMsg string) post.Attachment {
	return post.Attachment{}
}

type mockPollUserMapper struct {
	mapping map[string]string
}

func (m *mockPollUserMapper) GetKeepUsername(mattermostUsername string) (string, bool) {
	for mm, keep := range m.mapping {
		if mm == mattermostUsername {
			return keep, true
		}
	}
	return "", false
}

func (m *mockPollUserMapper) GetMattermostUsername(keepUsername string) (string, bool) {
	for mm, keep := range m.mapping {
		if keep == keepUsername {
			return mm, true
		}
	}
	return "", false
}

func setupPollAlertsUseCase() (*PollAlertsUseCase, *mockPollPostRepository, *mockPollKeepClient, *mockPollMattermostClient, *mockPollUserMapper) {
	postRepo := newMockPollPostRepository()
	keepClient := &mockPollKeepClient{}
	mmClient := &mockPollMattermostClient{}
	msgBuilder := &mockPollMessageBuilder{}
	userMapper := &mockPollUserMapper{mapping: make(map[string]string)}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	uc := NewPollAlertsUseCase(
		postRepo,
		keepClient,
		mmClient,
		msgBuilder,
		userMapper,
		"https://keep.example.com",
		"https://callback.example.com",
		1000,
		logger,
	)

	return uc, postRepo, keepClient, mmClient, userMapper
}

func TestPollAlertsUseCase_NoActivePosts(t *testing.T) {
	uc, _, _, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.False(t, mmClient.updatePostCalled)
}

func TestPollAlertsUseCase_FindAllActiveError(t *testing.T) {
	uc, postRepo, _, _, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	postRepo.findAllErr = errors.New("redis error")

	err := uc.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "find all active posts")
}

func TestPollAlertsUseCase_GetAlertsError(t *testing.T) {
	uc, postRepo, keepClient, _, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = p

	keepClient.getAlertsErr = errors.New("keep api error")

	err := uc.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get alerts from Keep")
}

func TestPollAlertsUseCase_AlertNotFoundInKeep(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = p

	// Keep returns different alert
	keepClient.alerts = []port.KeepAlert{
		{Fingerprint: "fp-other", Name: "Other Alert", Status: "firing", Severity: "high"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.False(t, mmClient.updatePostCalled, "should not update post if alert not found in Keep")
}

func TestPollAlertsUseCase_SkipResolvedAlert(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{Fingerprint: "fp-123", Name: "Test Alert", Status: "resolved", Severity: "high"},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.False(t, mmClient.updatePostCalled, "should skip resolved alerts")
}

func TestPollAlertsUseCase_NoAssigneeChange(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	p.SetLastKnownAssignee("existinguser")
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{
			Fingerprint: "fp-123",
			Name:        "Test Alert",
			Status:      "firing",
			Severity:    "high",
			Enrichments: map[string]string{"assignee": "existinguser"},
		},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.False(t, mmClient.updatePostCalled, "should not update when assignee unchanged")
}

func TestPollAlertsUseCase_DetectAssigneeChange(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	p.SetLastKnownAssignee("olduser")
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{
			Fingerprint: "fp-123",
			Name:        "Test Alert",
			Status:      "acknowledged",
			Severity:    "high",
			Enrichments: map[string]string{"assignee": "newuser"},
		},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled, "should update post when assignee changed")
	assert.Contains(t, mmClient.replyMessage, "newuser")
	assert.Contains(t, mmClient.replyMessage, "Keep UI")

	// Verify assignee was saved
	savedPost := postRepo.posts[fp.Value()]
	assert.Equal(t, "newuser", savedPost.LastKnownAssignee())
}

func TestPollAlertsUseCase_DetectAssigneeRemoval(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	p.SetLastKnownAssignee("previoususer")
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{
			Fingerprint: "fp-123",
			Name:        "Test Alert",
			Status:      "firing",
			Severity:    "high",
			Enrichments: map[string]string{}, // No assignee
		},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled, "should update post when assignee removed")
	assert.Contains(t, mmClient.replyMessage, "removed")

	// Verify empty assignee was saved
	savedPost := postRepo.posts[fp.Value()]
	assert.Equal(t, "", savedPost.LastKnownAssignee())
}

func TestPollAlertsUseCase_UserMapping(t *testing.T) {
	uc, postRepo, keepClient, mmClient, userMapper := setupPollAlertsUseCase()
	ctx := context.Background()

	// Set up user mapping: keep "john.doe" -> mattermost "johnd"
	userMapper.mapping["johnd"] = "john.doe"

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{
			Fingerprint: "fp-123",
			Name:        "Test Alert",
			Status:      "acknowledged",
			Severity:    "high",
			Enrichments: map[string]string{"assignee": "john.doe"},
		},
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)
	// Should use Mattermost username in reply
	assert.Contains(t, mmClient.replyMessage, "johnd")
}

func TestPollAlertsUseCase_UpdatePostError(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{
			Fingerprint: "fp-123",
			Name:        "Test Alert",
			Status:      "acknowledged",
			Severity:    "high",
			Enrichments: map[string]string{"assignee": "newuser"},
		},
	}

	mmClient.updatePostErr = errors.New("mattermost error")

	err := uc.Execute(ctx)

	// Should not return error, but continue processing
	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled)
	// Assignee should NOT be updated if post update failed
	assert.Equal(t, "", postRepo.posts[fp.Value()].LastKnownAssignee())
}

func TestPollAlertsUseCase_MultipleAlerts(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	// Set up 3 tracked posts
	fp1 := alert.RestoreFingerprint("fp-1")
	p1 := post.NewPost("post-1", "channel-1", fp1, "Alert 1", alert.RestoreSeverity("high"), time.Now())
	p1.SetLastKnownAssignee("user1")
	postRepo.posts[fp1.Value()] = p1

	fp2 := alert.RestoreFingerprint("fp-2")
	p2 := post.NewPost("post-2", "channel-1", fp2, "Alert 2", alert.RestoreSeverity("critical"), time.Now())
	p2.SetLastKnownAssignee("user2")
	postRepo.posts[fp2.Value()] = p2

	fp3 := alert.RestoreFingerprint("fp-3")
	p3 := post.NewPost("post-3", "channel-1", fp3, "Alert 3", alert.RestoreSeverity("warning"), time.Now())
	postRepo.posts[fp3.Value()] = p3

	keepClient.alerts = []port.KeepAlert{
		{Fingerprint: "fp-1", Name: "Alert 1", Status: "firing", Severity: "high", Enrichments: map[string]string{"assignee": "user1"}},             // unchanged
		{Fingerprint: "fp-2", Name: "Alert 2", Status: "acknowledged", Severity: "critical", Enrichments: map[string]string{"assignee": "newuser"}}, // changed
		{Fingerprint: "fp-3", Name: "Alert 3", Status: "resolved", Severity: "warning", Enrichments: map[string]string{}},                           // resolved - skip
	}

	err := uc.Execute(ctx)

	require.NoError(t, err)
	assert.True(t, mmClient.updatePostCalled, "should update at least one post")
	// fp-2 should have updated assignee
	assert.Equal(t, "newuser", postRepo.posts[fp2.Value()].LastKnownAssignee())
	// fp-1 should remain unchanged
	assert.Equal(t, "user1", postRepo.posts[fp1.Value()].LastKnownAssignee())
}

func TestPollAlertsUseCase_ContextCancellation(t *testing.T) {
	uc, postRepo, keepClient, _, _ := setupPollAlertsUseCase()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{Fingerprint: "fp-123", Name: "Test Alert", Status: "firing", Severity: "high"},
	}

	// Execute should handle cancelled context gracefully
	// The behavior depends on how underlying calls handle cancellation
	err := uc.Execute(ctx)

	// Context cancellation may cause FindAllActive or GetAlerts to fail
	// but the use case should not panic
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}
}

func TestPollAlertsUseCase_SaveFailureAfterUpdateSuccess(t *testing.T) {
	uc, postRepo, keepClient, mmClient, _ := setupPollAlertsUseCase()
	ctx := context.Background()

	fp := alert.RestoreFingerprint("fp-123")
	p := post.NewPost("post-1", "channel-1", fp, "Test Alert", alert.RestoreSeverity("high"), time.Now())
	p.SetLastKnownAssignee("olduser")
	postRepo.posts[fp.Value()] = p

	keepClient.alerts = []port.KeepAlert{
		{
			Fingerprint: "fp-123",
			Name:        "Test Alert",
			Status:      "acknowledged",
			Severity:    "high",
			Enrichments: map[string]string{"assignee": "newuser"},
		},
	}

	// Mattermost update succeeds, but repository save fails
	postRepo.saveErr = errors.New("redis connection error")

	err := uc.Execute(ctx)

	// Execute should not return error (continues processing other alerts)
	require.NoError(t, err)
	// Mattermost was updated (user sees the change)
	assert.True(t, mmClient.updatePostCalled)
	// Save was attempted but failed
	assert.True(t, postRepo.saveCalled)
	// Self-healing behavior: in-memory object was modified, but Redis wasn't updated.
	// On next poll cycle, data will be re-fetched from Redis (with old assignee),
	// and the change will be re-detected and re-applied.
	// This is acceptable eventual consistency behavior.
}
