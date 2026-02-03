package usecase

import "github.com/VictoriaMetrics/metrics"

var (
	alertReFireCounter  = metrics.NewCounter(`alerts_updated_total{action="re-fire"}`)
	alertResolveCounter = metrics.NewCounter(`alerts_updated_total{action="resolve"}`)
	alertAckCounter     = metrics.NewCounter(`alerts_updated_total{action="acknowledge"}`)
	alertUnackCounter   = metrics.NewCounter(`alerts_updated_total{action="unacknowledge"}`)

	alertsReceivedCounter = func(severity, status string) *metrics.Counter {
		return metrics.GetOrCreateCounter(`alerts_received_total{severity="` + severity + `",status="` + status + `"}`)
	}
	alertsPostedCounter = func(severity, channel string) *metrics.Counter {
		return metrics.GetOrCreateCounter(`alerts_posted_total{severity="` + severity + `",channel="` + channel + `"}`)
	}
	callbacksReceivedCounter = func(action string) *metrics.Counter {
		return metrics.GetOrCreateCounter(`callbacks_received_total{action="` + action + `"}`)
	}
)
