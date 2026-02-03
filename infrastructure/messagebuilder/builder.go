package messagebuilder

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type Builder struct {
	msgConfig port.MessageConfig
}

func NewBuilder(msgConfig port.MessageConfig) *Builder {
	return &Builder{msgConfig: msgConfig}
}

func (b *Builder) BuildFiringAttachment(a *alert.Alert, callbackURL, keepUIURL string) post.Attachment {
	severity := a.Severity().String()
	color := b.msgConfig.ColorForSeverity(severity)
	emoji := b.msgConfig.EmojiForSeverity(severity)

	title := fmt.Sprintf("%s %s | %s", emoji, strings.ToUpper(severity), a.Name())
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels())

	if a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	buttons := []post.Button{
		{
			ID:   "acknowledge",
			Name: "Acknowledge",
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					"action":      "acknowledge",
					"fingerprint": a.Fingerprint().Value(),
					"alert_name":  a.Name(),
					"severity":    severity,
				},
			},
		},
		{
			ID:   "resolve",
			Name: "Resolve",
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					"action":      "resolve",
					"fingerprint": a.Fingerprint().Value(),
					"alert_name":  a.Name(),
					"severity":    severity,
				},
			},
		},
	}

	return post.Attachment{
		Color:     color,
		Title:     title,
		TitleLink: titleLink,
		Fields:    fields,
		Actions:   buttons,
	}
}

func (b *Builder) BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment {
	severity := a.Severity().String()
	color := b.msgConfig.ColorForSeverity("acknowledged")

	title := fmt.Sprintf("👀 ACKNOWLEDGED | %s", a.Name())
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels())

	if a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	buttons := []post.Button{
		{
			ID:   "resolve",
			Name: "Resolve",
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					"action":      "resolve",
					"fingerprint": a.Fingerprint().Value(),
					"alert_name":  a.Name(),
					"severity":    severity,
				},
			},
		},
	}

	return post.Attachment{
		Color:     color,
		Title:     title,
		TitleLink: titleLink,
		Fields:    fields,
		Actions:   buttons,
	}
}

func (b *Builder) BuildResolvedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	color := b.msgConfig.ColorForSeverity("resolved")

	title := fmt.Sprintf("✅ RESOLVED | %s", a.Name())
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels())

	if a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	return post.Attachment{
		Color:     color,
		Title:     title,
		TitleLink: titleLink,
		Fields:    fields,
	}
}

func (b *Builder) buildFields(labels map[string]string) []post.AttachmentField {
	var fields []post.AttachmentField

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if b.msgConfig.IsLabelExcluded(key) {
			continue
		}
		if !b.msgConfig.IsLabelDisplayed(key) {
			continue
		}

		value := labels[key]
		if value == "" {
			continue
		}

		displayName := b.msgConfig.RenameLabel(key)
		fields = append(fields, post.AttachmentField{
			Title: displayName,
			Value: value,
			Short: true,
		})
	}

	return fields
}
