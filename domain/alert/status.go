package alert

import (
	"fmt"
	"strings"
)

type Status struct {
	value string
}

const (
	StatusFiring       = "firing"
	StatusResolved     = "resolved"
	StatusAcknowledged = "acknowledged"
	StatusSuppressed   = "suppressed"
	StatusPending      = "pending"
	StatusMaintenance  = "maintenance"
)

var validStatuses = map[string]bool{
	StatusFiring:       true,
	StatusResolved:     true,
	StatusAcknowledged: true,
	StatusSuppressed:   true,
	StatusPending:      true,
	StatusMaintenance:  true,
}

func NewStatus(value string) (Status, error) {
	normalized := strings.ToLower(value)
	if !validStatuses[normalized] {
		return Status{}, fmt.Errorf("%w: %s", ErrInvalidStatus, value)
	}
	return Status{value: normalized}, nil
}

func RestoreStatus(value string) Status {
	return Status{value: value}
}

func (s Status) Value() string {
	return s.value
}

func (s Status) String() string {
	return s.value
}

func (s Status) IsFiring() bool {
	return s.value == StatusFiring
}

func (s Status) IsResolved() bool {
	return s.value == StatusResolved
}

func (s Status) IsAcknowledged() bool {
	return s.value == StatusAcknowledged
}

func (s Status) IsSuppressed() bool {
	return s.value == StatusSuppressed
}

func (s Status) IsPending() bool {
	return s.value == StatusPending
}

func (s Status) IsMaintenance() bool {
	return s.value == StatusMaintenance
}
