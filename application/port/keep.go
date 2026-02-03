package port

import (
	"context"
	"time"
)

type KeepAlert struct {
	Fingerprint     string
	Name            string
	Status          string
	Severity        string
	Description     string
	Source          []string
	Labels          map[string]string
	FiringStartTime time.Time
}

type KeepClient interface {
	EnrichAlert(ctx context.Context, fingerprint, status string) error
	GetAlert(ctx context.Context, fingerprint string) (*KeepAlert, error)
}
