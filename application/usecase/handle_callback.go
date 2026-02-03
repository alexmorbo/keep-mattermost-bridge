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
	keepUIURL string,
	callbackURL string,
	logger *slog.Logger,
) *HandleCallbackUseCase {
	return &HandleCallbackUseCase{
		postRepo:    postRepo,
		keepClient:  keepClient,
		mmClient:    mmClient,
		msgBuilder:  msgBuilder,
		keepUIURL:   keepUIURL,
		callbackURL: callbackURL,
		logger:      logger,
	}
}

func (uc *HandleCallbackUseCase) Execute(ctx context.Context, input dto.MattermostCallbackInput) (*dto.CallbackOutput, error) {
	action := input.Context["action"]
	fingerprintStr := input.Context["fingerprint"]

	uc.logger.Info("Callback received",
		logger.ApplicationFields("callback_received",
			slog.String("action", action),
			slog.String("fingerprint", fingerprintStr),
			slog.String("user_id", input.UserID),
		),
	)

	validActions := map[string]bool{"acknowledge": true, "resolve": true, "unacknowledge": true}
	metricAction := "unknown"
	if validActions[action] {
		metricAction = action
	}
	callbacksReceivedCounter(metricAction).Inc()

	fingerprint, err := alert.NewFingerprint(fingerprintStr)
	if err != nil {
		return nil, fmt.Errorf("parse fingerprint: %w", err)
	}

	keepAlert, err := uc.keepClient.GetAlert(ctx, fingerprintStr)
	if err != nil {
		return nil, fmt.Errorf("get alert from keep: %w", err)
	}

	severity, err := alert.NewSeverity(keepAlert.Severity)
	if err != nil {
		return nil, fmt.Errorf("parse severity: %w", err)
	}

	username, err := uc.mmClient.GetUser(ctx, input.UserID)
	if err != nil {
		uc.logger.Warn("Failed to get username, using user_id",
			slog.String("user_id", input.UserID),
			slog.String("error", err.Error()),
		)
		username = input.UserID
	}

	statusStr := action
	if action == "acknowledge" {
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
	case "acknowledge":
		return uc.handleAcknowledge(ctx, a, fingerprint, username)
	case "resolve":
		return uc.handleResolve(ctx, a, fingerprint, username)
	case "unacknowledge":
		return uc.handleUnacknowledge(ctx, a, fingerprint, username)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func (uc *HandleCallbackUseCase) handleAcknowledge(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, username string) (*dto.CallbackOutput, error) {
	_ = ctx // ctx not needed for acknowledge, but kept for API consistency with handleResolve
	uc.wg.Add(1)
	go func() {
		defer uc.wg.Done()
		enrichCtx, enrichCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer enrichCancel()
		if err := uc.keepClient.EnrichAlert(enrichCtx, fingerprint.Value(), "acknowledged"); err != nil {
			uc.logger.Error("Failed to enrich alert in Keep",
				slog.String("fingerprint", fingerprint.Value()),
				slog.String("error", err.Error()),
			)
		}
	}()

	attachment := uc.msgBuilder.BuildAcknowledgedAttachment(a, uc.callbackURL, uc.keepUIURL, username)

	uc.logger.Info("Callback processed",
		logger.ApplicationFields("callback_processed",
			slog.String("action", "acknowledge"),
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("username", username),
		),
	)
	alertAckCounter.Inc()

	return &dto.CallbackOutput{
		Attachment: dto.NewAttachmentDTO(attachment),
		Ephemeral:  fmt.Sprintf("Alert acknowledged by @%s", username),
	}, nil
}

func (uc *HandleCallbackUseCase) handleResolve(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, username string) (*dto.CallbackOutput, error) {
	uc.wg.Add(1)
	go func() {
		defer uc.wg.Done()
		enrichCtx, enrichCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer enrichCancel()
		if err := uc.keepClient.EnrichAlert(enrichCtx, fingerprint.Value(), "resolved"); err != nil {
			uc.logger.Error("Failed to enrich alert in Keep",
				slog.String("fingerprint", fingerprint.Value()),
				slog.String("error", err.Error()),
			)
		}
	}()

	attachment := uc.msgBuilder.BuildResolvedAttachment(a, uc.keepUIURL)

	if err := uc.postRepo.Delete(ctx, fingerprint); err != nil {
		return nil, fmt.Errorf("delete post from store: %w", err)
	}

	uc.logger.Info("Callback processed",
		logger.ApplicationFields("callback_processed",
			slog.String("action", "resolve"),
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("username", username),
		),
	)
	alertResolveCounter.Inc()

	return &dto.CallbackOutput{
		Attachment: dto.NewAttachmentDTO(attachment),
		Ephemeral:  fmt.Sprintf("Alert resolved by @%s", username),
	}, nil
}

func (uc *HandleCallbackUseCase) handleUnacknowledge(ctx context.Context, a *alert.Alert, fingerprint alert.Fingerprint, username string) (*dto.CallbackOutput, error) {
	_ = ctx
	uc.wg.Add(1)
	go func() {
		defer uc.wg.Done()
		unenrichCtx, unenrichCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer unenrichCancel()
		if err := uc.keepClient.UnenrichAlert(unenrichCtx, fingerprint.Value()); err != nil {
			uc.logger.Error("Failed to unenrich alert in Keep",
				slog.String("fingerprint", fingerprint.Value()),
				slog.String("error", err.Error()),
			)
		}
	}()

	attachment := uc.msgBuilder.BuildFiringAttachment(a, uc.callbackURL, uc.keepUIURL)

	uc.logger.Info("Callback processed",
		logger.ApplicationFields("callback_processed",
			slog.String("action", "unacknowledge"),
			slog.String("fingerprint", fingerprint.Value()),
			slog.String("username", username),
		),
	)
	alertUnackCounter.Inc()

	return &dto.CallbackOutput{
		Attachment: dto.NewAttachmentDTO(attachment),
		Ephemeral:  fmt.Sprintf("Alert unacknowledged by @%s", username),
	}, nil
}

func (uc *HandleCallbackUseCase) Wait() {
	uc.wg.Wait()
}
