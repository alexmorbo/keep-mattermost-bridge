package alert

import (
	"fmt"
	"regexp"
)

var fingerprintRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type Fingerprint struct {
	value string
}

func NewFingerprint(value string) (Fingerprint, error) {
	if value == "" {
		return Fingerprint{}, fmt.Errorf("%w: empty value", ErrInvalidFingerprint)
	}
	if len(value) > 512 {
		return Fingerprint{}, fmt.Errorf("%w: exceeds max length 512", ErrInvalidFingerprint)
	}
	if !fingerprintRegex.MatchString(value) {
		return Fingerprint{}, fmt.Errorf("%w: contains invalid characters", ErrInvalidFingerprint)
	}
	return Fingerprint{value: value}, nil
}

func RestoreFingerprint(value string) Fingerprint {
	return Fingerprint{value: value}
}

func (f Fingerprint) Value() string {
	return f.value
}

func (f Fingerprint) String() string {
	return f.value
}

func (f Fingerprint) Equals(other Fingerprint) bool {
	return f.value == other.value
}
