package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
	"github.com/alexmorbo/keep-mattermost-bridge/pkg/logger"
)

type PollAlertsUseCase struct {
	postRepo    post.Repository
	keepClient  port.KeepClient
	mmClient    port.MattermostClient
	msgBuilder  port.MessageBuilder
	userMapper  port.UserMapper
	keepUIURL   string
	callbackURL string
	alertsLimit int
	logger      *slog.Logger
}

func NewPollAlertsUseCase(
	postRepo post.Repository,
	keepClient port.KeepClient,
	mmClient port.MattermostClient,
	msgBuilder port.MessageBuilder,
	userMapper port.UserMapper,
	keepUIURL string,
	callbackURL string,
	alertsLimit int,
	logger *slog.Logger,
) *PollAlertsUseCase {
	return &PollAlertsUseCase{
		postRepo:    postRepo,
		keepClient:  keepClient,
		mmClient:    mmClient,
		msgBuilder:  msgBuilder,
		userMapper:  userMapper,
		keepUIURL:   keepUIURL,
		callbackURL: callbackURL,
		alertsLimit: alertsLimit,
		logger:      logger,
	}
}

func (uc *PollAlertsUseCase) Execute(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		pollDurationSeconds.UpdateDuration(startTime)
	}()

	pollExecutionsCounter.Inc()

	trackedPosts, err := uc.postRepo.FindAllActive(ctx)
	if err != nil {
		pollErrorsCounter.Inc()
		return fmt.Errorf("find all active posts: %w", err)
	}

	pollActivePostsGauge.Set(float64(len(trackedPosts)))

	if len(trackedPosts) == 0 {
		uc.logger.Debug("No active posts to poll")
		return nil
	}

	keepAlerts, err := uc.keepClient.GetAlerts(ctx, uc.alertsLimit)
	if err != nil {
		pollErrorsCounter.Inc()
		return fmt.Errorf("get alerts from Keep: %w", err)
	}

	alertMap := make(map[string]port.KeepAlert, len(keepAlerts))
	for _, ka := range keepAlerts {
		alertMap[ka.Fingerprint] = ka
	}

	uc.logger.Debug("Polling for assignee changes",
		slog.Int("tracked_posts", len(trackedPosts)),
		slog.Int("keep_alerts", len(keepAlerts)),
	)

	for _, trackedPost := range trackedPosts {
		pollAlertsCheckedCounter.Inc()

		fingerprint := trackedPost.Fingerprint().Value()
		keepAlert, exists := alertMap[fingerprint]
		if !exists {
			uc.logger.Debug("Tracked alert not found in Keep (may be resolved)",
				slog.String("fingerprint", fingerprint),
			)
			continue
		}

		if keepAlert.Status == alert.StatusResolved {
			uc.logger.Debug("Skipping resolved alert",
				slog.String("fingerprint", fingerprint),
			)
			continue
		}

		currentAssignee := uc.resolveAssigneeUsername(keepAlert.Enrichments)
		lastKnownAssignee := trackedPost.LastKnownAssignee()

		if currentAssignee != lastKnownAssignee {
			if currentAssignee == "" {
				uc.logger.Info("Assignee removed via polling",
					logger.ApplicationFields("poll_assignee_removed",
						slog.String("fingerprint", fingerprint),
						slog.String("previous_assignee", lastKnownAssignee),
					),
				)
			} else {
				uc.logger.Info("Assignee change detected via polling",
					logger.ApplicationFields("poll_assignee_changed",
						slog.String("fingerprint", fingerprint),
						slog.String("previous_assignee", lastKnownAssignee),
						slog.String("new_assignee", currentAssignee),
					),
				)
			}
			pollAssigneeChangedCounter.Inc()

			if err := uc.handleAssigneeChange(ctx, trackedPost, keepAlert, currentAssignee); err != nil {
				uc.logger.Error("Failed to handle assignee change",
					logger.ApplicationFields("poll_assignee_change_failed",
						slog.String("fingerprint", fingerprint),
						slog.Any("error", err),
					),
				)
				pollErrorsCounter.Inc()
				continue
			}
		}
	}

	return nil
}

func (uc *PollAlertsUseCase) handleAssigneeChange(ctx context.Context, trackedPost *post.Post, keepAlert port.KeepAlert, newAssignee string) error {
	fingerprint := trackedPost.Fingerprint()

	severity, err := alert.NewSeverity(keepAlert.Severity)
	if err != nil {
		severity = trackedPost.Severity()
	}

	status, err := alert.NewStatus(keepAlert.Status)
	if err != nil {
		status = alert.RestoreStatus(alert.StatusFiring)
	}

	source := strings.Join(keepAlert.Source, ", ")

	a := alert.RestoreAlert(
		fingerprint,
		keepAlert.Name,
		severity,
		status,
		keepAlert.Description,
		source,
		keepAlert.Labels,
		trackedPost.FiringStartTime(),
	)

	var attachment post.Attachment
	var replyMsg string

	if newAssignee == "" {
		// Assignee was removed - show as firing alert
		attachment = uc.msgBuilder.BuildFiringAttachment(a, uc.callbackURL, uc.keepUIURL)
		replyMsg = "Assignee removed (via Keep UI)"
	} else {
		attachment = uc.msgBuilder.BuildAcknowledgedAttachment(a, uc.callbackURL, uc.keepUIURL, newAssignee)
		replyMsg = fmt.Sprintf("Assignee changed to @%s (via Keep UI)", newAssignee)
	}

	if err := uc.mmClient.UpdatePost(ctx, trackedPost.PostID(), attachment); err != nil {
		return fmt.Errorf("update mattermost post: %w", err)
	}

	if err := uc.mmClient.ReplyToThread(ctx, trackedPost.ChannelID(), trackedPost.PostID(), replyMsg); err != nil {
		uc.logger.Warn("Failed to reply to thread",
			slog.String("post_id", trackedPost.PostID()),
			slog.Any("error", err),
		)
	}

	trackedPost.SetLastKnownAssignee(newAssignee)
	trackedPost.Touch()
	if err := uc.postRepo.Save(ctx, fingerprint, trackedPost); err != nil {
		return fmt.Errorf("save post to store: %w", err)
	}

	return nil
}

func (uc *PollAlertsUseCase) resolveAssigneeUsername(enrichments map[string]string) string {
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
