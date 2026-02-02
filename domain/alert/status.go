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
)

var validStatuses = map[string]bool{
	StatusFiring:       true,
	StatusResolved:     true,
	StatusAcknowledged: true,
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
