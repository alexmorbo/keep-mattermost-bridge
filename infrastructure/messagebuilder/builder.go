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

	title := fmt.Sprintf("%s %s | %s", emoji, strings.ToUpper(severity), a.Name())
	if duration := formatDuration(a.FiringStartTime()); duration != "" {
		title = fmt.Sprintf("%s (%s)", title, duration)
	}
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
	if duration := formatDuration(a.FiringStartTime()); duration != "" {
		title = fmt.Sprintf("%s (%s)", title, duration)
	}
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels())

	if a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	buttons := []post.Button{
		{
			ID:   "unacknowledge",
			Name: "Unacknowledge",
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					"action":      "unacknowledge",
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

func (b *Builder) BuildResolvedAttachment(a *alert.Alert, keepUIURL string) post.Attachment {
	color := b.msgConfig.ColorForSeverity("resolved")

	title := fmt.Sprintf("✅ RESOLVED | %s", a.Name())
	if duration := formatDuration(a.FiringStartTime()); duration != "" {
		title = fmt.Sprintf("%s (%s)", title, duration)
	}
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
	var topologyLabels []string

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if b.msgConfig.IsLabelExcluded(key) {
			continue
		}

		value := labels[key]
		if value == "" {
			continue
		}

		// Collect topology labels separately
		if strings.HasPrefix(key, "topology_") || strings.Contains(key, "_topology_") {
			displayKey := strings.ReplaceAll(key, "_", ".")
			topologyLabels = append(topologyLabels, fmt.Sprintf("• %s: %s", displayKey, value))
			continue
		}

		if !b.msgConfig.IsLabelDisplayed(key) {
			continue
		}

		displayName := b.msgConfig.RenameLabel(key)
		fields = append(fields, post.AttachmentField{
			Title: displayName,
			Value: value,
			Short: true,
		})
	}

	// Add topology section if there are any topology labels
	if len(topologyLabels) > 0 {
		fields = append(fields, post.AttachmentField{
			Title: "Topology",
			Value: strings.Join(topologyLabels, "\n"),
			Short: false,
		})
	}

	return fields
}

func formatDuration(start time.Time) string {
	if start.IsZero() {
		return ""
	}

	d := time.Since(start)
	if d < 0 {
		return ""
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return "<1m"
	}
}
