package port

import (
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type MessageBuilder interface {
	BuildFiringAttachment(a *alert.Alert, callbackURL, keepUIURL string) post.Attachment
	BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment
	BuildResolvedAttachment(a *alert.Alert, keepUIURL string) post.Attachment
	BuildProcessingAttachment(attachmentJSON, action string) (post.Attachment, error)
	BuildErrorAttachment(alertName, fingerprint, keepUIURL, errorMsg string) post.Attachment
}
