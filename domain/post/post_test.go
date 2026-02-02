package post

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
)

// Post entity tests
func TestNewPost(t *testing.T) {
	postID := "post-123"
	channelID := "channel-456"
	fingerprint := alert.RestoreFingerprint("fp-789")
	alertName := "Test Alert"
	severity := alert.RestoreSeverity("critical")

	beforeCreate := time.Now()
	p := NewPost(postID, channelID, fingerprint, alertName, severity)
	afterCreate := time.Now()

	require.NotNil(t, p)
	assert.Equal(t, postID, p.PostID())
	assert.Equal(t, channelID, p.ChannelID())
	assert.Equal(t, fingerprint, p.Fingerprint())
	assert.Equal(t, alertName, p.AlertName())
	assert.Equal(t, severity, p.Severity())

	// Timestamps should be set to approximately now
	assert.True(t, p.CreatedAt().After(beforeCreate) || p.CreatedAt().Equal(beforeCreate))
	assert.True(t, p.CreatedAt().Before(afterCreate) || p.CreatedAt().Equal(afterCreate))

	assert.True(t, p.LastUpdated().After(beforeCreate) || p.LastUpdated().Equal(beforeCreate))
	assert.True(t, p.LastUpdated().Before(afterCreate) || p.LastUpdated().Equal(afterCreate))

	// CreatedAt and LastUpdated should be equal for new posts
	assert.Equal(t, p.CreatedAt(), p.LastUpdated())
}

func TestRestorePost(t *testing.T) {
	postID := "post-abc"
	channelID := "channel-def"
	fingerprint := alert.RestoreFingerprint("fp-ghi")
	alertName := "Restored Alert"
	severity := alert.RestoreSeverity("high")
	createdAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	lastUpdated := time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC)

	p := RestorePost(postID, channelID, fingerprint, alertName, severity, createdAt, lastUpdated)

	require.NotNil(t, p)
	assert.Equal(t, postID, p.PostID())
	assert.Equal(t, channelID, p.ChannelID())
	assert.Equal(t, fingerprint, p.Fingerprint())
	assert.Equal(t, alertName, p.AlertName())
	assert.Equal(t, severity, p.Severity())
	assert.Equal(t, createdAt, p.CreatedAt())
	assert.Equal(t, lastUpdated, p.LastUpdated())
}

func TestPostTouch(t *testing.T) {
	createdAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	lastUpdated := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	p := RestorePost("post-1", "channel-1", alert.RestoreFingerprint("fp-1"), "Alert", alert.RestoreSeverity("info"), createdAt, lastUpdated)

	// Verify initial state
	assert.Equal(t, createdAt, p.CreatedAt())
	assert.Equal(t, lastUpdated, p.LastUpdated())

	// Touch the post
	beforeTouch := time.Now()
	time.Sleep(1 * time.Millisecond) // Ensure time passes
	p.Touch()
	afterTouch := time.Now()

	// CreatedAt should remain unchanged
	assert.Equal(t, createdAt, p.CreatedAt())

	// LastUpdated should be updated to now
	assert.True(t, p.LastUpdated().After(beforeTouch))
	assert.True(t, p.LastUpdated().Before(afterTouch) || p.LastUpdated().Equal(afterTouch))

	// LastUpdated should be after the original value
	assert.True(t, p.LastUpdated().After(lastUpdated))
}

func TestPostGetters(t *testing.T) {
	postID := "post-xyz"
	channelID := "channel-uvw"
	fingerprint := alert.RestoreFingerprint("fp-rst")
	alertName := "Database Alert"
	severity := alert.RestoreSeverity("warning")
	createdAt := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	lastUpdated := time.Date(2024, 6, 15, 11, 45, 0, 0, time.UTC)

	p := RestorePost(postID, channelID, fingerprint, alertName, severity, createdAt, lastUpdated)

	t.Run("PostID getter", func(t *testing.T) {
		assert.Equal(t, postID, p.PostID())
	})

	t.Run("ChannelID getter", func(t *testing.T) {
		assert.Equal(t, channelID, p.ChannelID())
	})

	t.Run("Fingerprint getter", func(t *testing.T) {
		assert.Equal(t, fingerprint, p.Fingerprint())
	})

	t.Run("AlertName getter", func(t *testing.T) {
		assert.Equal(t, alertName, p.AlertName())
	})

	t.Run("Severity getter", func(t *testing.T) {
		assert.Equal(t, severity, p.Severity())
	})

	t.Run("CreatedAt getter", func(t *testing.T) {
		assert.Equal(t, createdAt, p.CreatedAt())
	})

	t.Run("LastUpdated getter", func(t *testing.T) {
		assert.Equal(t, lastUpdated, p.LastUpdated())
	})
}

// Attachment tests
func TestAttachment(t *testing.T) {
	t.Run("create attachment with all fields", func(t *testing.T) {
		attachment := Attachment{
			Color:     "#FF0000",
			Title:     "Alert Title",
			TitleLink: "https://example.com/alert",
			Text:      "Alert description text",
			Fields: []AttachmentField{
				{Title: "Severity", Value: "High", Short: true},
				{Title: "Source", Value: "Prometheus", Short: true},
			},
			Actions: []Button{
				{
					ID:   "ack",
					Name: "Acknowledge",
					Integration: ButtonIntegration{
						URL: "https://api.example.com/ack",
						Context: map[string]string{
							"alert_id": "123",
						},
					},
				},
			},
			Footer:     "Alert System",
			FooterIcon: "https://example.com/icon.png",
		}

		assert.Equal(t, "#FF0000", attachment.Color)
		assert.Equal(t, "Alert Title", attachment.Title)
		assert.Equal(t, "https://example.com/alert", attachment.TitleLink)
		assert.Equal(t, "Alert description text", attachment.Text)
		assert.Equal(t, 2, len(attachment.Fields))
		assert.Equal(t, "Severity", attachment.Fields[0].Title)
		assert.Equal(t, "High", attachment.Fields[0].Value)
		assert.True(t, attachment.Fields[0].Short)
		assert.Equal(t, 1, len(attachment.Actions))
		assert.Equal(t, "ack", attachment.Actions[0].ID)
		assert.Equal(t, "Alert System", attachment.Footer)
		assert.Equal(t, "https://example.com/icon.png", attachment.FooterIcon)
	})

	t.Run("create minimal attachment", func(t *testing.T) {
		attachment := Attachment{
			Text: "Simple alert",
		}

		assert.Equal(t, "Simple alert", attachment.Text)
		assert.Empty(t, attachment.Color)
		assert.Empty(t, attachment.Title)
		assert.Nil(t, attachment.Fields)
		assert.Nil(t, attachment.Actions)
	})
}

func TestAttachmentField(t *testing.T) {
	t.Run("create short field", func(t *testing.T) {
		field := AttachmentField{
			Title: "Status",
			Value: "Active",
			Short: true,
		}

		assert.Equal(t, "Status", field.Title)
		assert.Equal(t, "Active", field.Value)
		assert.True(t, field.Short)
	})

	t.Run("create long field", func(t *testing.T) {
		field := AttachmentField{
			Title: "Description",
			Value: "This is a long description that should not be displayed inline",
			Short: false,
		}

		assert.Equal(t, "Description", field.Title)
		assert.Equal(t, "This is a long description that should not be displayed inline", field.Value)
		assert.False(t, field.Short)
	})
}

// Button tests
func TestButton(t *testing.T) {
	t.Run("create button with integration", func(t *testing.T) {
		button := Button{
			ID:   "acknowledge",
			Name: "Acknowledge Alert",
			Integration: ButtonIntegration{
				URL: "https://api.example.com/acknowledge",
				Context: map[string]string{
					"fingerprint": "abc123",
					"action":      "ack",
				},
			},
		}

		assert.Equal(t, "acknowledge", button.ID)
		assert.Equal(t, "Acknowledge Alert", button.Name)
		assert.Equal(t, "https://api.example.com/acknowledge", button.Integration.URL)
		assert.Equal(t, 2, len(button.Integration.Context))
		assert.Equal(t, "abc123", button.Integration.Context["fingerprint"])
		assert.Equal(t, "ack", button.Integration.Context["action"])
	})

	t.Run("create button with empty context", func(t *testing.T) {
		button := Button{
			ID:   "resolve",
			Name: "Resolve",
			Integration: ButtonIntegration{
				URL:     "https://api.example.com/resolve",
				Context: map[string]string{},
			},
		}

		assert.Equal(t, "resolve", button.ID)
		assert.Equal(t, "Resolve", button.Name)
		assert.Equal(t, "https://api.example.com/resolve", button.Integration.URL)
		assert.NotNil(t, button.Integration.Context)
		assert.Equal(t, 0, len(button.Integration.Context))
	})

	t.Run("create button with nil context", func(t *testing.T) {
		button := Button{
			ID:   "snooze",
			Name: "Snooze",
			Integration: ButtonIntegration{
				URL:     "https://api.example.com/snooze",
				Context: nil,
			},
		}

		assert.Equal(t, "snooze", button.ID)
		assert.Equal(t, "Snooze", button.Name)
		assert.Equal(t, "https://api.example.com/snooze", button.Integration.URL)
		assert.Nil(t, button.Integration.Context)
	})
}

func TestButtonIntegration(t *testing.T) {
	t.Run("create integration with context", func(t *testing.T) {
		integration := ButtonIntegration{
			URL: "https://webhook.example.com",
			Context: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		}

		assert.Equal(t, "https://webhook.example.com", integration.URL)
		assert.Equal(t, 3, len(integration.Context))
		assert.Equal(t, "value1", integration.Context["key1"])
		assert.Equal(t, "value2", integration.Context["key2"])
		assert.Equal(t, "value3", integration.Context["key3"])
	})
}
