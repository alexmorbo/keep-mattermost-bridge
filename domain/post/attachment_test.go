package post

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachment_ToJSON_RoundTrip(t *testing.T) {
	original := Attachment{
		Color:     "#CC0000",
		Title:     "🔴 Critical Alert",
		TitleLink: "http://example.com/alert?id=123",
		Text:      "This is the alert description",
		Fields: []AttachmentField{
			{Title: "Severity", Value: "CRITICAL", Short: true},
			{Title: "Host", Value: "server-1", Short: true},
		},
		Actions: []Button{
			{ID: "ack", Name: "Acknowledge", Style: "default"},
		},
		Footer:     "Keep AIOps",
		FooterIcon: "http://example.com/icon.png",
	}

	jsonStr, err := original.ToJSON()
	require.NoError(t, err)
	require.NotEmpty(t, jsonStr)

	restored, err := AttachmentFromJSON(jsonStr)
	require.NoError(t, err)

	assert.Equal(t, original.Color, restored.Color)
	assert.Equal(t, original.Title, restored.Title)
	assert.Equal(t, original.TitleLink, restored.TitleLink)
	assert.Equal(t, original.Text, restored.Text)
	assert.Equal(t, original.Footer, restored.Footer)
	assert.Equal(t, original.FooterIcon, restored.FooterIcon)
	assert.Len(t, restored.Fields, 2)
	assert.Len(t, restored.Actions, 1)
}

func TestAttachment_ToJSON_EmptyAttachment(t *testing.T) {
	empty := Attachment{}

	jsonStr, err := empty.ToJSON()
	require.NoError(t, err)
	require.NotEmpty(t, jsonStr)

	restored, err := AttachmentFromJSON(jsonStr)
	require.NoError(t, err)

	assert.Equal(t, "", restored.Color)
	assert.Equal(t, "", restored.Title)
	assert.Nil(t, restored.Fields)
	assert.Nil(t, restored.Actions)
}

func TestAttachment_ToJSON_SpecialCharacters(t *testing.T) {
	special := Attachment{
		Title: "Alert with \"quotes\" and 'apostrophes'",
		Text:  "Line1\nLine2\tTabbed",
		Fields: []AttachmentField{
			{Title: "Unicode", Value: "日本語テスト émojis 🎉", Short: false},
			{Title: "Symbols", Value: "<script>alert('xss')</script>", Short: false},
		},
	}

	jsonStr, err := special.ToJSON()
	require.NoError(t, err)

	restored, err := AttachmentFromJSON(jsonStr)
	require.NoError(t, err)

	assert.Equal(t, special.Title, restored.Title)
	assert.Equal(t, special.Text, restored.Text)
	assert.Equal(t, special.Fields[0].Value, restored.Fields[0].Value)
	assert.Equal(t, special.Fields[1].Value, restored.Fields[1].Value)
}

func TestAttachmentFromJSON_InvalidJSON(t *testing.T) {
	_, err := AttachmentFromJSON("not valid json")
	require.Error(t, err)
}

func TestAttachmentFromJSON_EmptyString(t *testing.T) {
	_, err := AttachmentFromJSON("")
	require.Error(t, err)
}

func TestAttachmentFromJSON_WrongType(t *testing.T) {
	_, err := AttachmentFromJSON(`"just a string"`)
	require.Error(t, err)
}
