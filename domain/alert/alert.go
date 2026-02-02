package alert

import "fmt"

type Alert struct {
	fingerprint Fingerprint
	name        string
	severity    Severity
	status      Status
	description string
	source      string
	labels      map[string]string
}

func NewAlert(
	fingerprint Fingerprint,
	name string,
	severity Severity,
	status Status,
	description string,
	source string,
	labels map[string]string,
) (*Alert, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: empty name", ErrInvalidAlert)
	}
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	return &Alert{
		fingerprint: fingerprint,
		name:        name,
		severity:    severity,
		status:      status,
		description: description,
		source:      source,
		labels:      copied,
	}, nil
}

func RestoreAlert(
	fingerprint Fingerprint,
	name string,
	severity Severity,
	status Status,
	description string,
	source string,
	labels map[string]string,
) *Alert {
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	return &Alert{
		fingerprint: fingerprint,
		name:        name,
		severity:    severity,
		status:      status,
		description: description,
		source:      source,
		labels:      copied,
	}
}

func (a *Alert) Fingerprint() Fingerprint { return a.fingerprint }
func (a *Alert) Name() string             { return a.name }
func (a *Alert) Severity() Severity       { return a.severity }
func (a *Alert) Status() Status           { return a.status }
func (a *Alert) Description() string      { return a.description }
func (a *Alert) Source() string           { return a.source }

func (a *Alert) Labels() map[string]string {
	result := make(map[string]string, len(a.labels))
	for k, v := range a.labels {
		result[k] = v
	}
	return result
}
