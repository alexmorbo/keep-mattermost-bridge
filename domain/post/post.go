package post

import (
	"time"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
)

type Post struct {
	postID          string
	channelID       string
	fingerprint     alert.Fingerprint
	alertName       string
	severity        alert.Severity
	firingStartTime time.Time
	createdAt       time.Time
	lastUpdated     time.Time
}

func NewPost(postID, channelID string, fingerprint alert.Fingerprint, alertName string, severity alert.Severity, firingStartTime time.Time) *Post {
	now := time.Now()
	return &Post{
		postID:          postID,
		channelID:       channelID,
		fingerprint:     fingerprint,
		alertName:       alertName,
		severity:        severity,
		firingStartTime: firingStartTime,
		createdAt:       now,
		lastUpdated:     now,
	}
}

func RestorePost(postID, channelID string, fingerprint alert.Fingerprint, alertName string, severity alert.Severity, firingStartTime, createdAt, lastUpdated time.Time) *Post {
	return &Post{
		postID:          postID,
		channelID:       channelID,
		fingerprint:     fingerprint,
		alertName:       alertName,
		severity:        severity,
		firingStartTime: firingStartTime,
		createdAt:       createdAt,
		lastUpdated:     lastUpdated,
	}
}

func (p *Post) PostID() string                 { return p.postID }
func (p *Post) ChannelID() string              { return p.channelID }
func (p *Post) Fingerprint() alert.Fingerprint { return p.fingerprint }
func (p *Post) AlertName() string              { return p.alertName }
func (p *Post) Severity() alert.Severity       { return p.severity }
func (p *Post) FiringStartTime() time.Time     { return p.firingStartTime }
func (p *Post) CreatedAt() time.Time           { return p.createdAt }
func (p *Post) LastUpdated() time.Time         { return p.lastUpdated }

func (p *Post) Touch() {
	p.lastUpdated = time.Now()
}
