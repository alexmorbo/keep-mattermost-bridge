package alert

import "errors"

var (
	ErrInvalidFingerprint = errors.New("invalid fingerprint")
	ErrInvalidSeverity    = errors.New("invalid severity")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrInvalidAlert       = errors.New("invalid alert")
)
