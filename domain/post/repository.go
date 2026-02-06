package post

import (
	"context"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
)

type Repository interface {
	Save(ctx context.Context, fingerprint alert.Fingerprint, p *Post) error
	FindByFingerprint(ctx context.Context, fingerprint alert.Fingerprint) (*Post, error)
	FindAllActive(ctx context.Context) ([]*Post, error)
	Delete(ctx context.Context, fingerprint alert.Fingerprint) error
}
