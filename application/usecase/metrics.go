package usecase

import (
	"strconv"

	"github.com/VictoriaMetrics/metrics"
)

var (
	alertReFireCounter      = metrics.NewCounter(`alerts_updated_total{action="re-fire"}`)
	alertResolveCounter     = metrics.NewCounter(`alerts_updated_total{action="resolve"}`)
	alertAckCounter         = metrics.NewCounter(`alerts_updated_total{action="acknowledge"}`)
	alertUnackCounter       = metrics.NewCounter(`alerts_updated_total{action="unacknowledge"}`)
	alertSuppressedCounter  = metrics.NewCounter(`alerts_updated_total{action="suppressed"}`)
	alertPendingCounter     = metrics.NewCounter(`alerts_updated_total{action="pending"}`)
	alertMaintenanceCounter = metrics.NewCounter(`alerts_updated_total{action="maintenance"}`)

	alertsReceivedCounter = func(severity, status string) *metrics.Counter {
		return metrics.GetOrCreateCounter(`alerts_received_total{severity="` + severity + `",status="` + status + `"}`)
	}
	alertsPostedCounter = func(severity, channel string) *metrics.Counter {
		return metrics.GetOrCreateCounter(`alerts_posted_total{severity="` + severity + `",channel="` + channel + `"}`)
	}
	callbacksReceivedCounter = func(action string) *metrics.Counter {
		return metrics.GetOrCreateCounter(`callbacks_received_total{action="` + action + `"}`)
	}

	// Retry metrics for assignee fetching
	assigneeRetryAttempts = func(attempt int) *metrics.Counter {
		return metrics.GetOrCreateCounter(`assignee_retry_attempts_total{attempt="` + strconv.Itoa(attempt) + `"}`)
	}
	assigneeRetrySuccess   = metrics.NewCounter(`assignee_retry_result_total{result="success"}`)
	assigneeRetryExhausted = metrics.NewCounter(`assignee_retry_result_total{result="exhausted"}`)
	assigneeRetryError     = metrics.NewCounter(`assignee_retry_result_total{result="error"}`)

	// Polling metrics
	pollExecutionsCounter      = metrics.NewCounter(`poll_executions_total`)
	pollAlertsCheckedCounter   = metrics.NewCounter(`poll_alerts_checked_total`)
	pollAssigneeChangedCounter = metrics.NewCounter(`poll_assignee_changes_detected_total`)
	pollErrorsCounter          = metrics.NewCounter(`poll_errors_total`)
	pollActivePostsGauge       = metrics.NewGauge(`poll_active_posts_count`, nil)
	pollDurationSeconds        = metrics.NewHistogram(`poll_duration_seconds`)
)
