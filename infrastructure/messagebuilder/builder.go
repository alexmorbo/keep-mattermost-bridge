package messagebuilder

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

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

	pretext := fmt.Sprintf("%s **%s** | %s", emoji, strings.ToUpper(severity), a.Name())
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
		Color:      color,
		Pretext:    pretext,
		Title:      "View in Keep",
		TitleLink:  titleLink,
		Fields:     fields,
		Actions:    buttons,
		Footer:     b.msgConfig.FooterText() + " | " + time.Now().Format("2006-01-02 15:04:05 MST"),
		FooterIcon: b.msgConfig.FooterIconURL(),
	}
}

func (b *Builder) BuildAcknowledgedAttachment(a *alert.Alert, callbackURL, keepUIURL, username string) post.Attachment {
	severity := a.Severity().String()
	color := b.msgConfig.ColorForSeverity("acknowledged")

	pretext := fmt.Sprintf(":eyes: **ACKNOWLEDGED** | %s", a.Name())
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

	footer := fmt.Sprintf("%s | Acknowledged by @%s | %s",
		b.msgConfig.FooterText(),
		username,
		time.Now().Format("2006-01-02 15:04:05 MST"),
	)

	return post.Attachment{
		Color:      color,
		Pretext:    pretext,
		Title:      "View in Keep",
		TitleLink:  titleLink,
		Fields:     fields,
		Actions:    buttons,
		Footer:     footer,
		FooterIcon: b.msgConfig.FooterIconURL(),
	}
}

func (b *Builder) BuildResolvedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	color := b.msgConfig.ColorForSeverity("resolved")

	pretext := fmt.Sprintf(":white_check_mark: **RESOLVED** | %s", a.Name())
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels())

	if a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	footer := fmt.Sprintf("%s | Resolved at %s",
		b.msgConfig.FooterText(),
		time.Now().Format("2006-01-02 15:04:05 MST"),
	)

	return post.Attachment{
		Color:      color,
		Pretext:    pretext,
		Title:      "View in Keep",
		TitleLink:  titleLink,
		Fields:     fields,
		Footer:     footer,
		FooterIcon: b.msgConfig.FooterIconURL(),
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
