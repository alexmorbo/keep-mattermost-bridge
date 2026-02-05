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
	msgBuilder      port.MessageBuilder
	channelResolver port.ChannelResolver
	keepUIURL       string
	callbackURL     string
	logger          *slog.Logger
}

func NewHandleAlertUseCase(
	postRepo post.Repository,
	mmClient port.MattermostClient,
	msgBuilder port.MessageBuilder,
	channelResolver port.ChannelResolver,
	keepUIURL string,
	callbackURL string,
	logger *slog.Logger,
) *HandleAlertUseCase {
	return &HandleAlertUseCase{
		postRepo:        postRepo,
		mmClient:        mmClient,
		msgBuilder:      msgBuilder,
		channelResolver: channelResolver,
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

	return nil
}

func (uc *HandleAlertUseCase) handleFiring(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint) error {
	existingPost, err := uc.postRepo.FindByFingerprint(ctx, fingerprint)
	if err != nil && !errors.Is(err, post.ErrNotFound) {
		return fmt.Errorf("find existing post: %w", err)
	}

	attachment := uc.msgBuilder.BuildFiringAttachment(a, uc.callbackURL, uc.keepUIURL)

	channelID := uc.channelResolver.ChannelIDForSeverity(a.Severity().String())

	if existingPost != nil {
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

	attachment := uc.msgBuilder.BuildResolvedAttachment(resolvedAlert, uc.keepUIURL)

	if err := uc.mmClient.UpdatePost(ctx, existingPost.PostID(), attachment); err != nil {
		return fmt.Errorf("update post to resolved: %w", err)
	}

	if err := uc.postRepo.Delete(ctx, fingerprint); err != nil {
		return fmt.Errorf("delete post from store: %w", err)
	}

	uc.logger.Info("Alert resolved",
		logger.ApplicationFields("alert_resolved",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("post_id", existingPost.PostID()),
		),
	)
	alertResolveCounter.Inc()

	return nil
}
