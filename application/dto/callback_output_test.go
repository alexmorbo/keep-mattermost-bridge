package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

func TestNewAttachmentDTO(t *testing.T) {
	t.Run("converts attachment with all fields", func(t *testing.T) {
		attachment := post.Attachment{
			Color:     "#FF0000",
			Title:     "Test Alert",
			TitleLink: "https://example.com/alert",
			Text:      "Alert description",
			Fields: []post.AttachmentField{
				{Title: "Severity", Value: "critical", Short: true},
				{Title: "Source", Value: "prometheus", Short: true},
			},
			Actions: []post.Button{
				{
					ID:   "btn-1",
					Name: "Acknowledge",
					Integration: post.ButtonIntegration{
						URL:     "https://callback.example.com",
						Context: map[string]string{"action": "acknowledge"},
					},
				},
				{
					ID:   "btn-2",
					Name: "Resolve",
					Integration: post.ButtonIntegration{
						URL:     "https://callback.example.com",
						Context: map[string]string{"action": "resolve"},
					},
				},
			},
			Footer:     "Keep AIOps",
			FooterIcon: "https://example.com/icon.png",
		}

		result := NewAttachmentDTO(attachment)

		assert.Equal(t, "#FF0000", result.Color)
		assert.Equal(t, "Test Alert", result.Title)
		assert.Equal(t, "https://example.com/alert", result.TitleLink)
		assert.Equal(t, "Alert description", result.Text)
		assert.Equal(t, "Keep AIOps", result.Footer)
		assert.Equal(t, "https://example.com/icon.png", result.FooterIcon)

		assert.Len(t, result.Fields, 2)
		assert.Equal(t, "Severity", result.Fields[0].Title)
		assert.Equal(t, "critical", result.Fields[0].Value)
		assert.True(t, result.Fields[0].Short)
		assert.Equal(t, "Source", result.Fields[1].Title)
		assert.Equal(t, "prometheus", result.Fields[1].Value)

		assert.Len(t, result.Actions, 2)
		assert.Equal(t, "btn-1", result.Actions[0].ID)
		assert.Equal(t, "Acknowledge", result.Actions[0].Name)
		assert.Equal(t, "https://callback.example.com", result.Actions[0].Integration.URL)
		assert.Equal(t, "acknowledge", result.Actions[0].Integration.Context["action"])
		assert.Equal(t, "btn-2", result.Actions[1].ID)
		assert.Equal(t, "Resolve", result.Actions[1].Name)
	})

	t.Run("converts attachment with empty fields and actions", func(t *testing.T) {
		attachment := post.Attachment{
			Color: "#00FF00",
			Title: "Simple Alert",
		}

		result := NewAttachmentDTO(attachment)

		assert.Equal(t, "#00FF00", result.Color)
		assert.Equal(t, "Simple Alert", result.Title)
		assert.Empty(t, result.TitleLink)
		assert.Empty(t, result.Text)
		assert.Empty(t, result.Footer)
		assert.Empty(t, result.FooterIcon)
		assert.Len(t, result.Fields, 0)
		assert.Len(t, result.Actions, 0)
	})

	t.Run("converts attachment with nil context in button", func(t *testing.T) {
		attachment := post.Attachment{
			Actions: []post.Button{
				{
					ID:   "btn-1",
					Name: "Action",
					Integration: post.ButtonIntegration{
						URL:     "https://example.com",
						Context: nil,
					},
				},
			},
		}

		result := NewAttachmentDTO(attachment)

		assert.Len(t, result.Actions, 1)
		assert.Nil(t, result.Actions[0].Integration.Context)
	})
}
