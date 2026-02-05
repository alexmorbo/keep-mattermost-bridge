package port

import (
	"context"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type MattermostClient interface {
	CreatePost(ctx context.Context, channelID string, attachment post.Attachment) (string, error)
	UpdatePost(ctx context.Context, postID string, attachment post.Attachment) error
	ReplyToThread(ctx context.Context, channelID, rootID, message string) error
	GetUser(ctx context.Context, userID string) (string, error)
}
