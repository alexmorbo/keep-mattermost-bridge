package port

import "context"

type KeepClient interface {
	EnrichAlert(ctx context.Context, fingerprint, status string) error
}
