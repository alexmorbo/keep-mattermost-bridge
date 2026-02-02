package alert

import (
	"fmt"
	"strings"
)

type Severity struct {
	value string
}

const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

var validSeverities = map[string]bool{
	SeverityCritical: true,
	SeverityHigh:     true,
	SeverityWarning:  true,
	SeverityInfo:     true,
}

func NewSeverity(value string) (Severity, error) {
	normalized := strings.ToLower(value)
	if !validSeverities[normalized] {
		return Severity{}, fmt.Errorf("%w: %s", ErrInvalidSeverity, value)
	}
	return Severity{value: normalized}, nil
}

func RestoreSeverity(value string) Severity {
	return Severity{value: value}
}

func (s Severity) Value() string {
	return s.value
}

func (s Severity) String() string {
	return s.value
}

func (s Severity) IsCritical() bool {
	return s.value == SeverityCritical
}

func (s Severity) IsHigh() bool {
	return s.value == SeverityHigh
}

func (s Severity) IsWarning() bool {
	return s.value == SeverityWarning
}

func (s Severity) IsInfo() bool {
	return s.value == SeverityInfo
}
