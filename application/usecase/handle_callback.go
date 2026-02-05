package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/alexmorbo/keep-mattermost-bridge/application/dto"
	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

type HandleCallbackUseCase struct {
	postRepo    post.Repository
	keepClient  port.KeepClient
	mmClient    port.MattermostClient
	msgBuilder  port.MessageBuilder
	userMapper  port.UserMapper
	keepUIURL   string
	callbackURL string
	logger      *slog.Logger
	wg          sync.WaitGroup
}

func NewHandleCallbackUseCase(
	postRepo post.Repository,
	keepClient port.KeepClient,
	mmClient port.MattermostClient,
	msgBuilder port.MessageBuilder,
	userMapper port.UserMapper,
	keepUIURL string,
	callbackURL string,
	logger *slog.Logger,
) *HandleCallbackUseCase {
	return &HandleCallbackUseCase{
		postRepo:    postRepo,
		keepClient:  keepClient,
		mmClient:    mmClient,
		msgBuilder:  msgBuilder,
		userMapper:  userMapper,
		keepUIURL:   keepUIURL,
		callbackURL: callbackURL,
		logger:      logger,
	}
}

func (uc *HandleCallbackUseCase) ExecuteImmediate(input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
	action := input.Context[post.ContextKeyAction]
	fingerprintStr := input.Context[post.ContextKeyFingerprint]
	alertName := input.Context[post.ContextKeyAlertName]
	attachmentJSON := input.Context[post.ContextKeyAttachmentJSON]

	uc.logger.Info("Callback received (immediate phase)",
		logger.ApplicationFields("callback_received",
			slog.String("action", action),
			slog.String("fingerprint", fingerprintStr),
			slog.String("user_id", input.UserID),
			slog.String("post_id", input.PostID),
		),
	)

	validActions := map[string]bool{
		post.ActionAcknowledge:   true,
		post.ActionResolve:       true,
		post.ActionUnacknowledge: true,
	}
	metricAction := "unknown"
	if validActions[action] {
		metricAction = action
	}
	callbacksReceivedCounter(metricAction).Inc()

	if _, err := alert.NewFingerprint(fingerprintStr); err != nil {
		return nil, fmt.Errorf("parse fingerprint: %w", err)
	}

	if alertName == "" {
		return nil, fmt.Errorf("missing required context field: alert_name")
	}

	if attachmentJSON == "" {
		return nil, fmt.Errorf("missing required context field: attachment_json")
	}

	processingAttachment, err := uc.msgBuilder.BuildProcessingAttachment(attachmentJSON, action)
	if err != nil {
		return nil, fmt.Errorf("build processing attachment: %w", err)
	}

	return &dto.CallbackOutput{
		Attachment: dto.NewAttachmentDTO(processingAttachment),
	}, nil
}

func (uc *HandleCallbackUseCase) ExecuteAsync(input dto.MattermostCallbackInput) {
	action := input.Context[post.ContextKeyAction]
	fingerprintStr := input.Context[post.ContextKeyFingerprint]
	alertName := input.Context[post.ContextKeyAlertName]

	uc.wg.Add(1)
	go func() {
		defer uc.wg.Done()

		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fingerprint, err := alert.NewFingerprint(fingerprintStr)
		if err != nil {
			uc.logger.Error("Failed to parse fingerprint in async phase",
				slog.String("fingerprint", fingerprintStr),
				slog.String("error", err.Error()),
			)
			uc.updatePostWithError(asyncCtx, input.PostID, alertName, fingerprintStr, "Invalid fingerprint")
			return
		}

		keepAlert, err := uc.keepClient.GetAlert(asyncCtx, fingerprintStr)
		if err != nil {
			uc.logger.Error("Failed to get alert from keep in async phase",
				slog.String("fingerprint", fingerprintStr),
				slog.String("error", err.Error()),
			)
			uc.updatePostWithError(asyncCtx, input.PostID, alertName, fingerprintStr, "Failed to get alert data")
			return
		}

		severity, err := alert.NewSeverity(keepAlert.Severity)
		if err != nil {
			uc.logger.Error("Failed to parse severity in async phase",
				slog.String("severity", keepAlert.Severity),
				slog.String("error", err.Error()),
			)
			uc.updatePostWithError(asyncCtx, input.PostID, alertName, fingerprintStr, "Invalid severity")
			return
		}

		username, err := uc.mmClient.GetUser(asyncCtx, input.UserID)
		if err != nil {
			uc.logger.Warn("Failed to get username, using user_id",
				slog.String("user_id", input.UserID),
				slog.String("error", err.Error()),
			)
			username = input.UserID
		}

		statusStr := action
		if action == post.ActionAcknowledge {
			statusStr = alert.StatusAcknowledged
		}

		source := strings.Join(keepAlert.Source, ", ")

		a := alert.RestoreAlert(
			fingerprint,
			keepAlert.Name,
			severity,
			alert.RestoreStatus(statusStr),
			keepAlert.Description,
			source,
			keepAlert.Labels,
			keepAlert.FiringStartTime,
		)

		switch action {
		case post.ActionAcknowledge:
			uc.handleAcknowledgeAsync(asyncCtx, a, fingerprint, username, input.PostID, input.ChannelID)
		case post.ActionResolve:
			uc.handleResolveAsync(asyncCtx, a, fingerprint, username, input.PostID, input.ChannelID)
		case post.ActionUnacknowledge:
			uc.handleUnacknowledgeAsync(asyncCtx, a, fingerprint, username, input.PostID, input.ChannelID)
		default:
			uc.logger.Error("Unknown action in async phase",
				slog.String("action", action),
			)
			uc.updatePostWithError(asyncCtx, input.PostID, alertName, fingerprintStr, "Unknown action")
		}
	}()
}

func (uc *HandleCallbackUseCase) updatePostWithError(ctx context.Context, postID, alertName, fingerprint, errorMsg string) {
	attachment := uc.msgBuilder.BuildErrorAttachment(alertName, fingerprint, uc.keepUIURL, errorMsg)
	if err := uc.mmClient.UpdatePost(ctx, postID, attachment); err != nil {
		uc.logger.Error("Failed to update post with error state",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}
}

func (uc *HandleCallbackUseCase) buildEnrichments(status, mattermostUsername string) map[string]string {
	enrichments := map[string]string{"status": status}
	if keepUser, ok := uc.userMapper.GetKeepUsername(mattermostUsername); ok && keepUser != "" {
		enrichments["assignee"] = strings.TrimSpace(keepUser)
		uc.logger.Debug("Mapped Mattermost user to Keep user",
			slog.String("mattermost_user", mattermostUsername),
			slog.String("keep_user", keepUser),
		)
	}
	return enrichments
}

func (uc *HandleCallbackUseCase) handleAcknowledgeAsync(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, username, postID, channelID string) {
	enrichments := uc.buildEnrichments("acknowledged", username)

	if err := uc.keepClient.EnrichAlert(ctx, fingerprint.Value(), enrichments); err != nil {
		uc.logger.Error("Failed to enrich alert in Keep",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("error", err.Error()),
		)
	}

	attachment := uc.msgBuilder.BuildAcknowledgedAttachment(a, uc.callbackURL, uc.keepUIURL, username)

	if err := uc.mmClient.UpdatePost(ctx, postID, attachment); err != nil {
		uc.logger.Error("Failed to update post",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}

	replyMsg := fmt.Sprintf("Acknowledged by @%s", username)
	if err := uc.mmClient.ReplyToThread(ctx, channelID, postID, replyMsg); err != nil {
		uc.logger.Error("Failed to reply to thread",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}

	uc.logger.Info("Callback processed (async)",
		logger.ApplicationFields("callback_processed_async",
			slog.String("action", "acknowledge"),
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("username", username),
		),
	)
	alertAckCounter.Inc()
}

func (uc *HandleCallbackUseCase) handleResolveAsync(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, username, postID, channelID string) {
	enrichments := uc.buildEnrichments("resolved", username)

	if err := uc.keepClient.EnrichAlert(ctx, fingerprint.Value(), enrichments); err != nil {
		uc.logger.Error("Failed to enrich alert in Keep",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("error", err.Error()),
		)
	}

	attachment := uc.msgBuilder.BuildResolvedAttachment(a, uc.keepUIURL)

	if err := uc.mmClient.UpdatePost(ctx, postID, attachment); err != nil {
		uc.logger.Error("Failed to update post",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}

	replyMsg := fmt.Sprintf("Resolved by @%s", username)
	if err := uc.mmClient.ReplyToThread(ctx, channelID, postID, replyMsg); err != nil {
		uc.logger.Error("Failed to reply to thread",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}

	if err := uc.postRepo.Delete(ctx, fingerprint); err != nil {
		uc.logger.Error("Failed to delete post from store",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("error", err.Error()),
		)
	}

	uc.logger.Info("Callback processed (async)",
		logger.ApplicationFields("callback_processed_async",
			slog.String("action", "resolve"),
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("username", username),
		),
	)
	alertResolveCounter.Inc()
}

func (uc *HandleCallbackUseCase) handleUnacknowledgeAsync(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, username, postID, channelID string) {
	if err := uc.keepClient.UnenrichAlert(ctx, fingerprint.Value()); err != nil {
		uc.logger.Error("Failed to unenrich alert in Keep",
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("error", err.Error()),
		)
	}

	attachment := uc.msgBuilder.BuildFiringAttachment(a, uc.callbackURL, uc.keepUIURL)

	if err := uc.mmClient.UpdatePost(ctx, postID, attachment); err != nil {
		uc.logger.Error("Failed to update post",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}

	replyMsg := fmt.Sprintf("Unacknowledged by @%s", username)
	if err := uc.mmClient.ReplyToThread(ctx, channelID, postID, replyMsg); err != nil {
		uc.logger.Error("Failed to reply to thread",
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
		)
	}

	uc.logger.Info("Callback processed (async)",
		logger.ApplicationFields("callback_processed_async",
			slog.String("action", "unacknowledge"),
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("username", username),
		),
	)
	alertUnackCounter.Inc()
}

func (uc *HandleCallbackUseCase) Wait() {
	uc.wg.Wait()
}
