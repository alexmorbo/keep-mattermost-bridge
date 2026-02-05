package messagebuilder

import (
	"fmt"
	"log/slog"
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

	title := fmt.Sprintf("%s %s", emoji, a.Name())
	if duration := formatDuration(a.FiringStartTime()); duration != "" {
		title = fmt.Sprintf("%s (%s)", title, duration)
	}
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels(), severity)

	if b.msgConfig.ShowDescriptionField() && a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	attachmentWithoutButtons := post.Attachment{
		Color:     color,
		Title:     title,
		TitleLink: titleLink,
		Fields:    fields,
	}

	attachmentJSON, err := attachmentWithoutButtons.ToJSON()
	if err != nil {
		slog.Error("Failed to serialize attachment to JSON", slog.String("error", err.Error()))
		attachmentJSON = ""
	}

	buttons := []post.Button{
		{
			ID:    post.ActionAcknowledge,
			Name:  "Acknowledge",
			Style: post.ButtonStyleDefault,
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					post.ContextKeyAction:         post.ActionAcknowledge,
					post.ContextKeyFingerprint:    a.Fingerprint().Value(),
					post.ContextKeyAlertName:      a.Name(),
					post.ContextKeySeverity:       severity,
					post.ContextKeyAttachmentJSON: attachmentJSON,
				},
			},
		},
		{
			ID:    post.ActionResolve,
			Name:  "Resolve",
			Style: post.ButtonStyleSuccess,
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					post.ContextKeyAction:         post.ActionResolve,
					post.ContextKeyFingerprint:    a.Fingerprint().Value(),
					post.ContextKeyAlertName:      a.Name(),
					post.ContextKeySeverity:       severity,
					post.ContextKeyAttachmentJSON: attachmentJSON,
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

	title := fmt.Sprintf("👀 %s", a.Name())
	if duration := formatDuration(a.FiringStartTime()); duration != "" {
		title = fmt.Sprintf("%s (%s)", title, duration)
	}
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels(), severity)

	if b.msgConfig.ShowDescriptionField() && a.Description() != "" {
		fields = append([]post.AttachmentField{
			{Title: "Description", Value: a.Description(), Short: false},
		}, fields...)
	}

	attachmentWithoutButtons := post.Attachment{
		Color:     color,
		Title:     title,
		TitleLink: titleLink,
		Fields:    fields,
	}

	attachmentJSON, err := attachmentWithoutButtons.ToJSON()
	if err != nil {
		slog.Error("Failed to serialize attachment to JSON", slog.String("error", err.Error()))
		attachmentJSON = ""
	}

	buttons := []post.Button{
		{
			ID:    post.ActionUnacknowledge,
			Name:  "Unacknowledge",
			Style: post.ButtonStyleDefault,
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					post.ContextKeyAction:         post.ActionUnacknowledge,
					post.ContextKeyFingerprint:    a.Fingerprint().Value(),
					post.ContextKeyAlertName:      a.Name(),
					post.ContextKeySeverity:       severity,
					post.ContextKeyAttachmentJSON: attachmentJSON,
				},
			},
		},
		{
			ID:    post.ActionResolve,
			Name:  "Resolve",
			Style: post.ButtonStyleSuccess,
			Integration: post.ButtonIntegration{
				URL: callbackURL,
				Context: map[string]string{
					post.ContextKeyAction:         post.ActionResolve,
					post.ContextKeyFingerprint:    a.Fingerprint().Value(),
					post.ContextKeyAlertName:      a.Name(),
					post.ContextKeySeverity:       severity,
					post.ContextKeyAttachmentJSON: attachmentJSON,
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
	severity := a.Severity().String()
	color := b.msgConfig.ColorForSeverity("resolved")

	title := fmt.Sprintf("✅ %s", a.Name())
	if duration := formatDuration(a.FiringStartTime()); duration != "" {
		title = fmt.Sprintf("%s (%s)", title, duration)
	}
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(a.Fingerprint().Value()))

	fields := b.buildFields(a.Labels(), severity)

	if b.msgConfig.ShowDescriptionField() && a.Description() != "" {
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

func (b *Builder) BuildProcessingAttachment(attachmentJSON, action string) (post.Attachment, error) {
	attachment, err := post.AttachmentFromJSON(attachmentJSON)
	if err != nil {
		return post.Attachment{}, fmt.Errorf("deserialize attachment: %w", err)
	}

	var style string
	switch action {
	case post.ActionResolve:
		style = post.ButtonStyleSuccess
	default:
		style = post.ButtonStyleDefault
	}

	attachment.Actions = []post.Button{
		{
			ID:    "processing",
			Name:  "Processing...",
			Style: style,
		},
	}

	return *attachment, nil
}

func (b *Builder) BuildErrorAttachment(alertName, fingerprint, keepUIURL, errorMsg string) post.Attachment {
	titleLink := fmt.Sprintf("%s/alerts/feed?fingerprint=%s", keepUIURL, url.QueryEscape(fingerprint))

	buttons := []post.Button{
		{
			ID:    "error",
			Name:  "Error: " + errorMsg,
			Style: post.ButtonStyleDanger,
		},
	}

	return post.Attachment{
		Color:     "#FF0000",
		Title:     alertName,
		TitleLink: titleLink,
		Actions:   buttons,
	}
}

func (b *Builder) buildFields(labels map[string]string, severity string) []post.AttachmentField {
	var displayFields []post.AttachmentField
	groupBuckets := make(map[string][]string)
	var ungroupedLabels []string

	groups := b.msgConfig.GetLabelGroups()
	groupingEnabled := b.msgConfig.IsLabelGroupingEnabled()
	threshold := b.msgConfig.GetLabelGroupingThreshold()

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

		if b.msgConfig.IsLabelDisplayed(key) {
			displayName := b.msgConfig.RenameLabel(key)
			displayFields = append(displayFields, post.AttachmentField{
				Title: displayName,
				Value: value,
				Short: true,
			})
			continue
		}

		if groupingEnabled {
			groupName := b.matchLabelToGroup(key, groups)
			if groupName != "" {
				formattedKey := b.formatLabelKey(key, groups)
				groupBuckets[groupName] = append(groupBuckets[groupName], fmt.Sprintf(" %s: `%s`", formattedKey, value))
			} else {
				ungroupedLabels = append(ungroupedLabels, fmt.Sprintf(" %s: `%s`", key, value))
			}
		}
	}

	var severityField post.AttachmentField
	showSeverity := b.msgConfig.ShowSeverityField()
	severityPosition := b.msgConfig.SeverityFieldPosition()

	if showSeverity {
		severityField = post.AttachmentField{
			Title: "Severity",
			Value: strings.ToUpper(severity),
			Short: true,
		}
	}

	var result []post.AttachmentField

	if showSeverity && severityPosition == post.SeverityPositionFirst {
		result = append(result, severityField)
	}

	result = append(result, displayFields...)

	if showSeverity && severityPosition == post.SeverityPositionAfterDisplay {
		result = append(result, severityField)
	}

	if groupingEnabled {
		sortedGroups := b.sortGroupsByPriority(groups)
		for _, group := range sortedGroups {
			bucket := groupBuckets[group.GroupName]
			if len(bucket) >= threshold {
				result = append(result, post.AttachmentField{
					Title: group.GroupName,
					Value: strings.Join(bucket, "\n"),
					Short: true,
				})
			} else {
				ungroupedLabels = append(ungroupedLabels, bucket...)
			}
		}

		if len(ungroupedLabels) > 0 {
			result = append(result, post.AttachmentField{
				Title: "Labels",
				Value: strings.Join(ungroupedLabels, "\n"),
				Short: true,
			})
		}
	}

	if showSeverity && severityPosition == post.SeverityPositionLast {
		result = append(result, severityField)
	}

	return result
}

func (b *Builder) matchLabelToGroup(key string, groups []port.LabelGroupConfig) string {
	for _, group := range groups {
		for _, prefix := range group.Prefixes {
			if strings.HasPrefix(key, prefix) {
				return group.GroupName
			}
		}
	}
	return ""
}

func (b *Builder) formatLabelKey(key string, groups []port.LabelGroupConfig) string {
	for _, group := range groups {
		for _, prefix := range group.Prefixes {
			if strings.HasPrefix(key, prefix) {
				return strings.TrimPrefix(key, prefix)
			}
		}
	}
	return key
}

func (b *Builder) sortGroupsByPriority(groups []port.LabelGroupConfig) []port.LabelGroupConfig {
	sorted := make([]port.LabelGroupConfig, len(groups))
	copy(sorted, groups)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	return sorted
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
