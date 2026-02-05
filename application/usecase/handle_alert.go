package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

type HandleAlertUseCase struct {
	postRepo        post.Repository
	mmClient        port.MattermostClient
	keepClient      port.KeepClient
	msgBuilder      port.MessageBuilder
	channelResolver port.ChannelResolver
	userMapper      port.UserMapper
	keepUIURL       string
	callbackURL     string
	logger          *slog.Logger
}

func NewHandleAlertUseCase(
	postRepo post.Repository,
	mmClient port.MattermostClient,
	keepClient port.KeepClient,
	msgBuilder port.MessageBuilder,
	channelResolver port.ChannelResolver,
	userMapper port.UserMapper,
	keepUIURL string,
	callbackURL string,
	logger *slog.Logger,
) *HandleAlertUseCase {
	return &HandleAlertUseCase{
		postRepo:        postRepo,
		mmClient:        mmClient,
		keepClient:      keepClient,
		msgBuilder:      msgBuilder,
		channelResolver: channelResolver,
		userMapper:      userMapper,
		keepUIURL:       keepUIURL,
		callbackURL:     callbackURL,
		logger:          logger,
	}
}

func (uc *HandleAlertUseCase) Execute(ctx context.Context, input dto.KeepAlertInput) error {
	fingerprint, err := alert.NewFingerprint(input.Fingerprint)
	if err != nil {
		return fmt.Errorf("parse fingerprint: %w", err)
	}

	severity, err := alert.NewSeverity(input.Severity)
	if err != nil {
		return fmt.Errorf("parse severity: %w", err)
	}

	status, err := alert.NewStatus(input.Status)
	if err != nil {
		return fmt.Errorf("parse status: %w", err)
	}

	source := strings.Join(input.Source, ", ")

	var firingStartTime time.Time
	if input.FiringStartTime != "" {
		var parseErr error
		firingStartTime, parseErr = time.Parse(time.RFC3339, input.FiringStartTime)
		if parseErr != nil {
			uc.logger.Warn("Failed to parse firingStartTime, using zero value",
				slog.String("value", input.FiringStartTime),
				slog.String("error", parseErr.Error()),
			)
		}
	}

	a, err := alert.NewAlert(fingerprint, input.Name, severity, status, input.Description, source, input.Labels, firingStartTime)
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}

	uc.logger.Info("Alert received",
		logger.ApplicationFields("alert_received",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("severity", severity.String()),
			slog.String("status", status.String()),
			slog.String("name", input.Name),
		),
	)
	alertsReceivedCounter(severity.String(), status.String()).Inc()

	if status.IsFiring() {
		return uc.handleFiring(ctx, a, fingerprint)
	}

	if status.IsResolved() {
		return uc.handleResolved(ctx, a, fingerprint)
	}

	if status.IsAcknowledged() {
		return uc.handleAcknowledged(ctx, a, fingerprint)
	}

	return nil
}

func (uc *HandleAlertUseCase) handleFiring(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint) error {
	existingPost, err := uc.postRepo.FindByFingerprint(ctx, fingerprint)
	if err != nil && !errors.Is(err, post.ErrNotFound) {
		return fmt.Errorf("find existing post: %w", err)
	}

	channelID := uc.channelResolver.ChannelIDForSeverity(a.Severity().String())

	if existingPost == nil {
		return uc.createFiringPost(ctx, a, fingerprint, channelID)
	}

	keepAlert, err := uc.keepClient.GetAlert(ctx, fingerprint.Value())
	if err != nil {
		uc.logger.Warn("Failed to get alert from Keep, proceeding without enrichments",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("error", err.Error()),
		)
	}

	var wasAcknowledged bool
	var assignee string
	if keepAlert != nil {
		assignee = uc.resolveAssigneeUsername(keepAlert.Enrichments)
		// Check both enrichment status and alert status from Keep
		// Keep may report acknowledged in either field depending on the source
		if keepAlert.Enrichments != nil {
			wasAcknowledged = keepAlert.Enrichments["status"] == "acknowledged"
		}
		wasAcknowledged = wasAcknowledged || keepAlert.Status == "acknowledged"
	}

	if wasAcknowledged || assignee != "" {
		alertWithStoredTime := alert.RestoreAlert(
			fingerprint, a.Name(), a.Severity(), a.Status(),
			a.Description(), a.Source(), a.Labels(),
			existingPost.FiringStartTime(),
		)
		attachment := uc.msgBuilder.BuildAcknowledgedAttachment(alertWithStoredTime, uc.callbackURL, uc.keepUIURL, assignee)

		if err := uc.mmClient.UpdatePost(ctx, existingPost.PostID(), attachment); err != nil {
			return fmt.Errorf("update post to acknowledged: %w", err)
		}

		var msg string
		if assignee != "" {
			msg = fmt.Sprintf("⚠️ Alert re-fired. Still acknowledged by @%s", assignee)
		} else {
			msg = "⚠️ Alert re-fired while acknowledged"
		}
		if err := uc.mmClient.ReplyToThread(ctx, existingPost.ChannelID(), existingPost.PostID(), msg); err != nil {
			uc.logger.Warn("Failed to reply to thread",
				slog.String("post_id", existingPost.PostID()),
				slog.String("error", err.Error()),
			)
		}

		uc.logger.Info("Acknowledged alert re-fired",
			logger.ApplicationFields("alert_refire_acknowledged",
				slog.String("fingerprint", fingerprint.Value()),
				slog.String("assignee", assignee),
			),
		)

		existingPost.Touch()
		if err := uc.postRepo.Save(ctx, fingerprint, existingPost); err != nil {
			return fmt.Errorf("update post in store: %w", err)
		}

		alertReFireCounter.Inc()
		return nil
	}

	alertWithStoredTime := alert.RestoreAlert(
		fingerprint, a.Name(), a.Severity(), a.Status(),
		a.Description(), a.Source(), a.Labels(),
		existingPost.FiringStartTime(),
	)
	attachment := uc.msgBuilder.BuildFiringAttachment(alertWithStoredTime, uc.callbackURL, uc.keepUIURL)

	if err := uc.mmClient.UpdatePost(ctx, existingPost.PostID(), attachment); err != nil {
		return fmt.Errorf("update existing post: %w", err)
	}

	existingPost.Touch()
	if err := uc.postRepo.Save(ctx, fingerprint, existingPost); err != nil {
		return fmt.Errorf("update post in store: %w", err)
	}

	uc.logger.Info("Alert updated (re-fire)",
		logger.ApplicationFields("alert_updated",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("post_id", existingPost.PostID()),
			slog.String("action", "re-fire"),
		),
	)
	alertReFireCounter.Inc()
	return nil
}

func (uc *HandleAlertUseCase) createFiringPost(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, channelID string) error {
	attachment := uc.msgBuilder.BuildFiringAttachment(a, uc.callbackURL, uc.keepUIURL)

	postID, err := uc.mmClient.CreatePost(ctx, channelID, attachment)
	if err != nil {
		return fmt.Errorf("create mattermost post: %w", err)
	}

	newPost := post.NewPost(postID, channelID, fingerprint, a.Name(), a.Severity(), a.FiringStartTime())
	if err := uc.postRepo.Save(ctx, fingerprint, newPost); err != nil {
		return fmt.Errorf("save post to store: %w", err)
	}

	uc.logger.Info("Alert posted to Mattermost",
		logger.ApplicationFields("alert_posted",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("severity", a.Severity().String()),
			slog.String("channel_id", channelID),
			slog.String("post_id", postID),
		),
	)
	alertsPostedCounter(a.Severity().String(), channelID).Inc()

	return nil
}

func (uc *HandleAlertUseCase) handleResolved(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint) error {
	existingPost, err := uc.postRepo.FindByFingerprint(ctx, fingerprint)
	if err != nil {
		if errors.Is(err, post.ErrNotFound) {
			uc.logger.Warn("Resolved alert without existing post",
				logger.ApplicationFields("alert_resolved",
					slog.String("fingerprint", fingerprint.Value()),
					slog.String("status", "no_existing_post"),
				),
			)
			return nil
		}
		return fmt.Errorf("find existing post: %w", err)
	}

	keepAlert, err := uc.keepClient.GetAlert(ctx, fingerprint.Value())
	if err != nil {
		uc.logger.Warn("Failed to get alert from Keep, proceeding without enrichments",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("error", err.Error()),
		)
	}

	var assignee string
	if keepAlert != nil {
		assignee = uc.resolveAssigneeUsername(keepAlert.Enrichments)
	}

	resolvedAlert := alert.RestoreAlert(
		fingerprint,
		a.Name(),
		a.Severity(),
		a.Status(),
		a.Description(),
		a.Source(),
		a.Labels(),
		existingPost.FiringStartTime(),
	)

	attachment := uc.msgBuilder.BuildResolvedAttachment(resolvedAlert, uc.keepUIURL, assignee)

	if err := uc.mmClient.UpdatePost(ctx, existingPost.PostID(), attachment); err != nil {
		return fmt.Errorf("update post to resolved: %w", err)
	}

	if assignee != "" {
		msg := fmt.Sprintf("✅ Alert automatically resolved. Was acknowledged by @%s", assignee)
		if err := uc.mmClient.ReplyToThread(ctx, existingPost.ChannelID(), existingPost.PostID(), msg); err != nil {
			uc.logger.Warn("Failed to reply to thread",
				slog.String("post_id", existingPost.PostID()),
				slog.String("error", err.Error()),
			)
		}
	}

	if err := uc.postRepo.Delete(ctx, fingerprint); err != nil {
		return fmt.Errorf("delete post from store: %w", err)
	}

	uc.logger.Info("Alert resolved",
		logger.ApplicationFields("alert_resolved",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("post_id", existingPost.PostID()),
			slog.String("assignee", assignee),
		),
	)
	alertResolveCounter.Inc()

	return nil
}

func (uc *HandleAlertUseCase) handleAcknowledged(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint) error {
	existingPost, err := uc.postRepo.FindByFingerprint(ctx, fingerprint)
	if err != nil {
		if errors.Is(err, post.ErrNotFound) {
			return uc.createAcknowledgedPost(ctx, a, fingerprint)
		}
		return fmt.Errorf("find existing post: %w", err)
	}

	// Fetch assignee from Keep with retry - enrichments may not be available immediately
	// due to race condition between callback setting enrichments and webhook arriving
	assignee := uc.fetchAssigneeWithRetry(ctx, fingerprint.Value())

	alertWithStoredTime := alert.RestoreAlert(
		fingerprint, a.Name(), a.Severity(), a.Status(),
		a.Description(), a.Source(), a.Labels(),
		existingPost.FiringStartTime(),
	)
	attachment := uc.msgBuilder.BuildAcknowledgedAttachment(alertWithStoredTime, uc.callbackURL, uc.keepUIURL, assignee)

	if err := uc.mmClient.UpdatePost(ctx, existingPost.PostID(), attachment); err != nil {
		return fmt.Errorf("update post to acknowledged: %w", err)
	}

	uc.logger.Info("Alert acknowledged (from Keep)",
		logger.ApplicationFields("alert_acknowledged",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("post_id", existingPost.PostID()),
			slog.String("assignee", assignee),
		),
	)
	alertAckCounter.Inc()

	return nil
}

func (uc *HandleAlertUseCase) createAcknowledgedPost(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint) error {
	// Fetch assignee from Keep with retry - enrichments may not be available immediately
	assignee := uc.fetchAssigneeWithRetry(ctx, fingerprint.Value())

	attachment := uc.msgBuilder.BuildAcknowledgedAttachment(a, uc.callbackURL, uc.keepUIURL, assignee)

	channelID := uc.channelResolver.ChannelIDForSeverity(a.Severity().String())

	postID, err := uc.mmClient.CreatePost(ctx, channelID, attachment)
	if err != nil {
		return fmt.Errorf("create mattermost post: %w", err)
	}

	newPost := post.NewPost(postID, channelID, fingerprint, a.Name(), a.Severity(), a.FiringStartTime())
	if err := uc.postRepo.Save(ctx, fingerprint, newPost); err != nil {
		return fmt.Errorf("save post to store: %w", err)
	}

	uc.logger.Info("Acknowledged alert posted to Mattermost",
		logger.ApplicationFields("alert_posted_acknowledged",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("severity", a.Severity().String()),
			slog.String("channel_id", channelID),
			slog.String("post_id", postID),
			slog.String("assignee", assignee),
		),
	)
	alertsPostedCounter(a.Severity().String(), channelID).Inc()

	return nil
}

// resolveAssigneeUsername converts Keep username from enrichments to Mattermost username for display.
// Falls back to Keep username if no reverse mapping exists.
func (uc *HandleAlertUseCase) resolveAssigneeUsername(enrichments map[string]string) string {
	if enrichments == nil {
		return ""
	}
	keepUser := enrichments["assignee"]
	if keepUser == "" {
		return ""
	}
	if mmUser, ok := uc.userMapper.GetMattermostUsername(keepUser); ok {
		return mmUser
	}
	return keepUser
}

// fetchAssigneeWithRetry fetches assignee from Keep API with exponential backoff retry.
// This handles the race condition where webhook arrives before enrichments are set.
func (uc *HandleAlertUseCase) fetchAssigneeWithRetry(ctx context.Context, fingerprint string) string {
	// Exponential backoff: 100ms, 200ms, 400ms (total max ~700ms)
	retryDelays := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}

	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		assigneeRetryAttempts(attempt + 1).Inc()

		keepAlert, err := uc.keepClient.GetAlert(ctx, fingerprint)
		if err != nil {
			uc.logger.Warn("Failed to get alert from Keep",
				slog.String("fingerprint", fingerprint),
				slog.String("error", err.Error()),
				slog.Int("attempt", attempt+1),
			)
			assigneeRetryError.Inc()
			return ""
		}

		assignee := uc.resolveAssigneeUsername(keepAlert.Enrichments)
		if assignee != "" {
			if attempt > 0 {
				uc.logger.Debug("Assignee found after retry",
					slog.String("fingerprint", fingerprint),
					slog.Int("attempt", attempt+1),
					slog.String("assignee", assignee),
				)
			}
			assigneeRetrySuccess.Inc()
			return assignee
		}

		// Assignee not set yet, wait and retry (unless last attempt)
		if attempt < len(retryDelays) {
			uc.logger.Debug("Assignee not found, retrying with backoff",
				slog.String("fingerprint", fingerprint),
				slog.Int("attempt", attempt+1),
				slog.Duration("delay", retryDelays[attempt]),
			)
			select {
			case <-ctx.Done():
				assigneeRetryError.Inc()
				return ""
			case <-time.After(retryDelays[attempt]):
				// continue to next attempt
			}
		}
	}

	uc.logger.Debug("Assignee not found after retries",
		slog.String("fingerprint", fingerprint),
		slog.Int("total_attempts", len(retryDelays)+1),
	)
	assigneeRetryExhausted.Inc()
	return ""
}
