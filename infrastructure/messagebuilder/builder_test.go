package messagebuilder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
	"github.com/alexmorbo/keep-mattermost-bridge/infrastructure/config"
)

func TestBuildFiringAttachment(t *testing.T) {
	tests := []struct {
		name           string
		fileConfig     *config.FileConfig
		alertSeverity  string
		alertName      string
		alertDesc      string
		labels         map[string]string
		expectedColor  string
		expectedEmoji  string
		expectedFields int
		hasButtons     bool
	}{
		{
			name: "critical alert with description and labels",
			fileConfig: &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"critical": "#CC0000"},
					Emoji:  map[string]string{"critical": "🔴"},
					Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
				},
				Labels: config.LabelsConfig{
					Display:  []string{"host", "job"},
					Exclude:  []string{},
					Rename:   map[string]string{"host": "Server"},
					Grouping: config.LabelGroupingConfig{},
				},
			},
			alertSeverity:  "critical",
			alertName:      "High CPU Usage",
			alertDesc:      "CPU usage exceeded 90%",
			labels:         map[string]string{"host": "server-1", "job": "monitoring"},
			expectedColor:  "#CC0000",
			expectedEmoji:  "🔴",
			expectedFields: 4,
			hasButtons:     true,
		},
		{
			name: "warning alert without description",
			fileConfig: &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"warning": "#EDA200"},
					Emoji:  map[string]string{"warning": "🟡"},
					Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
				},
				Labels: config.LabelsConfig{
					Display:  []string{"service"},
					Exclude:  []string{},
					Rename:   map[string]string{},
					Grouping: config.LabelGroupingConfig{},
				},
			},
			alertSeverity:  "warning",
			alertName:      "Disk Space Low",
			alertDesc:      "",
			labels:         map[string]string{"service": "api", "ignored": "value"},
			expectedColor:  "#EDA200",
			expectedEmoji:  "🟡",
			expectedFields: 2,
			hasButtons:     true,
		},
		{
			name: "info alert with excluded labels",
			fileConfig: &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"info": "#0066FF"},
					Emoji:  map[string]string{"info": "🔵"},
					Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
				},
				Labels: config.LabelsConfig{
					Display:  []string{},
					Exclude:  []string{"internal"},
					Rename:   map[string]string{},
					Grouping: config.LabelGroupingConfig{},
				},
			},
			alertSeverity:  "info",
			alertName:      "Service Started",
			alertDesc:      "Service started successfully",
			labels:         map[string]string{"host": "server-1", "internal": "skip-me"},
			expectedColor:  "#0066FF",
			expectedEmoji:  "🔵",
			expectedFields: 3,
			hasButtons:     true,
		},
		{
			name: "high severity with renamed labels",
			fileConfig: &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"high": "#FF6600"},
					Emoji:  map[string]string{"high": "🟠"},
					Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
				},
				Labels: config.LabelsConfig{
					Display:  []string{},
					Exclude:  []string{},
					Rename:   map[string]string{"job": "Job Name", "host": "Hostname"},
					Grouping: config.LabelGroupingConfig{},
				},
			},
			alertSeverity:  "high",
			alertName:      "Database Connection Failed",
			alertDesc:      "Cannot connect to database",
			labels:         map[string]string{"job": "db-monitor", "host": "db-01"},
			expectedColor:  "#FF6600",
			expectedEmoji:  "🟠",
			expectedFields: 4,
			hasButtons:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(tt.fileConfig)

			severity, err := alert.NewSeverity(tt.alertSeverity)
			require.NoError(t, err)

			fingerprint := alert.RestoreFingerprint("test-fingerprint-123")
			status := alert.RestoreStatus(alert.StatusFiring)

			testAlert := alert.RestoreAlert(
				fingerprint,
				tt.alertName,
				severity,
				status,
				tt.alertDesc,
				"prometheus",
				tt.labels,
				time.Time{},
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			assert.Equal(t, tt.expectedColor, attachment.Color, "color mismatch")
			assert.Contains(t, attachment.Title, tt.expectedEmoji, "emoji not in title")
			assert.Contains(t, attachment.Title, tt.alertName, "alert name not in title")
			assert.NotContains(t, attachment.Title, tt.alertSeverity, "severity should not be in title")
			assert.Contains(t, attachment.TitleLink, "http://keep.ui/alerts/feed?fingerprint=test-fingerprint-123")
			assert.Equal(t, tt.expectedFields, len(attachment.Fields), "fields count mismatch")

			// Find Severity field (may not be first if Description is present)
			var foundSeverity bool
			for _, field := range attachment.Fields {
				if field.Title == "Severity" {
					foundSeverity = true
					assert.True(t, field.Short, "Severity field should be short")
					break
				}
			}
			assert.True(t, foundSeverity, "should have Severity field")

			if tt.hasButtons {
				assert.Len(t, attachment.Actions, 2, "should have 2 buttons")
				assert.Equal(t, "acknowledge", attachment.Actions[0].ID)
				assert.Equal(t, "Acknowledge", attachment.Actions[0].Name)
				assert.Equal(t, "resolve", attachment.Actions[1].ID)
				assert.Equal(t, "Resolve", attachment.Actions[1].Name)
				assert.Equal(t, "http://callback.url", attachment.Actions[0].Integration.URL)
				assert.Equal(t, "test-fingerprint-123", attachment.Actions[0].Integration.Context["fingerprint"])
			}

		})
	}
}

func TestBuildAcknowledgedAttachment(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"acknowledged": "#FFA500"},
			Emoji:  map[string]string{},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Display:  []string{},
			Exclude:  []string{},
			Rename:   map[string]string{},
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, err := alert.NewSeverity("critical")
	require.NoError(t, err)

	fingerprint := alert.RestoreFingerprint("ack-fingerprint-456")
	status := alert.RestoreStatus(alert.StatusAcknowledged)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Test Alert",
		severity,
		status,
		"Test description",
		"prometheus",
		map[string]string{"env": "production"},
		time.Time{},
	)

	attachment := builder.BuildAcknowledgedAttachment(testAlert, "http://callback.url", "http://keep.ui", "john.doe")

	assert.Equal(t, "#FFA500", attachment.Color, "should have orange color")
	assert.Contains(t, attachment.Title, "👀")
	assert.Contains(t, attachment.Title, "Test Alert")
	assert.Contains(t, attachment.TitleLink, "http://keep.ui/alerts/feed?fingerprint=ack-fingerprint-456")

	assert.Len(t, attachment.Actions, 2, "should have Unacknowledge and Resolve buttons")
	assert.Equal(t, "unacknowledge", attachment.Actions[0].ID)
	assert.Equal(t, "Unacknowledge", attachment.Actions[0].Name)
	assert.Equal(t, "http://callback.url", attachment.Actions[0].Integration.URL)
	assert.Equal(t, "ack-fingerprint-456", attachment.Actions[0].Integration.Context["fingerprint"])
	assert.Equal(t, "unacknowledge", attachment.Actions[0].Integration.Context["action"])
	assert.Equal(t, "resolve", attachment.Actions[1].ID)
	assert.Equal(t, "Resolve", attachment.Actions[1].Name)
}

func TestBuildResolvedAttachment(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"resolved": "#00CC00"},
			Emoji:  map[string]string{},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Display:  []string{},
			Exclude:  []string{},
			Rename:   map[string]string{},
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, err := alert.NewSeverity("high")
	require.NoError(t, err)

	fingerprint := alert.RestoreFingerprint("resolved-fingerprint-789")
	status := alert.RestoreStatus(alert.StatusResolved)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Resolved Alert",
		severity,
		status,
		"This alert was resolved",
		"prometheus",
		map[string]string{"service": "api"},
		time.Time{},
	)

	attachment := builder.BuildResolvedAttachment(testAlert, "http://keep.ui", "")

	assert.Equal(t, "#00CC00", attachment.Color, "should have green color")
	assert.Contains(t, attachment.Title, "✅")
	assert.Contains(t, attachment.Title, "Resolved Alert")
	assert.Contains(t, attachment.TitleLink, "http://keep.ui/alerts/feed?fingerprint=resolved-fingerprint-789")

	assert.Len(t, attachment.Actions, 0, "should have no buttons")
}

func TestBuildResolvedAttachmentWithAcknowledgedBy(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"resolved": "#00CC00"},
			Emoji:  map[string]string{},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Display:  []string{},
			Exclude:  []string{},
			Rename:   map[string]string{},
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, err := alert.NewSeverity("high")
	require.NoError(t, err)

	fingerprint := alert.RestoreFingerprint("resolved-fingerprint-789")
	status := alert.RestoreStatus(alert.StatusResolved)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Resolved Alert",
		severity,
		status,
		"This alert was resolved",
		"prometheus",
		map[string]string{"service": "api"},
		time.Time{},
	)

	attachment := builder.BuildResolvedAttachment(testAlert, "http://keep.ui", "john.doe")

	assert.Equal(t, "#00CC00", attachment.Color, "should have green color")
	assert.Contains(t, attachment.Title, "✅")
	assert.Contains(t, attachment.Title, "Resolved Alert")
	assert.Equal(t, "Was acknowledged by @john.doe", attachment.Footer)
	assert.Equal(t, "https://test.com/icon.png", attachment.FooterIcon)
}

func TestBuildAcknowledgedAttachmentWithFooter(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"acknowledged": "#FFA500"},
			Emoji:  map[string]string{},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Display:  []string{},
			Exclude:  []string{},
			Rename:   map[string]string{},
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, err := alert.NewSeverity("critical")
	require.NoError(t, err)

	fingerprint := alert.RestoreFingerprint("ack-fingerprint-456")
	status := alert.RestoreStatus(alert.StatusAcknowledged)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Test Alert",
		severity,
		status,
		"Test description",
		"prometheus",
		map[string]string{"env": "production"},
		time.Time{},
	)

	attachment := builder.BuildAcknowledgedAttachment(testAlert, "http://callback.url", "http://keep.ui", "john.doe")

	assert.Equal(t, "#FFA500", attachment.Color, "should have orange color")
	assert.Contains(t, attachment.Title, "👀")
	assert.Contains(t, attachment.Title, "Test Alert")
	assert.Equal(t, "Acknowledged by @john.doe", attachment.Footer)
	assert.Equal(t, "https://test.com/icon.png", attachment.FooterIcon)
}

func TestBuildFieldsFiltering(t *testing.T) {
	tests := []struct {
		name          string
		displayLabels []string
		excludeLabels []string
		renameLabels  map[string]string
		inputLabels   map[string]string
		expectedCount int
		checkTitles   []string
	}{
		{
			name:          "exclude specific labels",
			displayLabels: []string{},
			excludeLabels: []string{"internal", "debug"},
			renameLabels:  map[string]string{},
			inputLabels: map[string]string{
				"host":     "server-1",
				"internal": "value",
				"debug":    "true",
				"env":      "prod",
			},
			expectedCount: 3,
			checkTitles:   []string{"Severity", "host", "env"},
		},
		{
			name:          "display only specific labels",
			displayLabels: []string{"host", "service"},
			excludeLabels: []string{},
			renameLabels:  map[string]string{},
			inputLabels: map[string]string{
				"host":    "server-1",
				"service": "api",
				"region":  "us-east",
				"env":     "prod",
			},
			expectedCount: 3,
			checkTitles:   []string{"Severity", "host", "service"},
		},
		{
			name:          "rename labels",
			displayLabels: []string{},
			excludeLabels: []string{},
			renameLabels: map[string]string{
				"host": "Server Name",
				"env":  "Environment",
			},
			inputLabels: map[string]string{
				"host": "server-1",
				"env":  "production",
			},
			expectedCount: 3,
			checkTitles:   []string{"Severity", "Server Name", "Environment"},
		},
		{
			name:          "empty label values are filtered",
			displayLabels: []string{},
			excludeLabels: []string{},
			renameLabels:  map[string]string{},
			inputLabels: map[string]string{
				"host":  "server-1",
				"empty": "",
				"env":   "prod",
			},
			expectedCount: 3,
			checkTitles:   []string{"Severity", "host", "env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileConfig := &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"info": "#0066FF"},
					Emoji:  map[string]string{"info": "🔵"},
					Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
				},
				Labels: config.LabelsConfig{
					Display:  tt.displayLabels,
					Exclude:  tt.excludeLabels,
					Rename:   tt.renameLabels,
					Grouping: config.LabelGroupingConfig{},
				},
			}

			builder := NewBuilder(fileConfig)

			severity, err := alert.NewSeverity("info")
			require.NoError(t, err)

			fingerprint := alert.RestoreFingerprint("test-fp")
			status := alert.RestoreStatus(alert.StatusFiring)

			testAlert := alert.RestoreAlert(
				fingerprint,
				"Test Alert",
				severity,
				status,
				"",
				"prometheus",
				tt.inputLabels,
				time.Time{},
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			assert.Equal(t, tt.expectedCount, len(attachment.Fields), "fields count mismatch")

			fieldTitles := make([]string, len(attachment.Fields))
			for i, field := range attachment.Fields {
				fieldTitles[i] = field.Title
			}

			for _, expectedTitle := range tt.checkTitles {
				assert.Contains(t, fieldTitles, expectedTitle, "expected title not found in fields")
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		expected string
	}{
		{
			name:     "zero time returns empty string",
			start:    time.Time{},
			expected: "",
		},
		{
			name:     "future time returns empty string",
			start:    time.Now().Add(1 * time.Hour),
			expected: "",
		},
		{
			name:     "less than 1 minute ago",
			start:    time.Now().Add(-30 * time.Second),
			expected: "<1m",
		},
		{
			name:     "45 minutes ago",
			start:    time.Now().Add(-45 * time.Minute),
			expected: "45m",
		},
		{
			name:     "2 hours 15 minutes ago",
			start:    time.Now().Add(-2*time.Hour - 15*time.Minute),
			expected: "2h 15m",
		},
		{
			name:     "3 days 12 hours ago",
			start:    time.Now().Add(-3*24*time.Hour - 12*time.Hour),
			expected: "3d 12h",
		},
		{
			name:     "exactly 1 hour",
			start:    time.Now().Add(-1 * time.Hour),
			expected: "1h 0m",
		},
		{
			name:     "exactly 1 day",
			start:    time.Now().Add(-24 * time.Hour),
			expected: "1d 0h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.start)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDifferentSeveritiesProduceDifferentColorsAndEmojis(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{
				"critical": "#CC0000",
				"high":     "#FF6600",
				"warning":  "#EDA200",
				"info":     "#0066FF",
			},
			Emoji: map[string]string{
				"critical": "🔴",
				"high":     "🟠",
				"warning":  "🟡",
				"info":     "🔵",
			},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severities := []struct {
		severity      string
		expectedColor string
		expectedEmoji string
	}{
		{"critical", "#CC0000", "🔴"},
		{"high", "#FF6600", "🟠"},
		{"warning", "#EDA200", "🟡"},
		{"info", "#0066FF", "🔵"},
	}

	for _, sv := range severities {
		t.Run(sv.severity, func(t *testing.T) {
			severity, err := alert.NewSeverity(sv.severity)
			require.NoError(t, err)

			fingerprint := alert.RestoreFingerprint("test-fp")
			status := alert.RestoreStatus(alert.StatusFiring)

			testAlert := alert.RestoreAlert(
				fingerprint,
				"Test Alert",
				severity,
				status,
				"",
				"prometheus",
				map[string]string{},
				time.Time{},
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			assert.Equal(t, sv.expectedColor, attachment.Color)
			assert.Contains(t, attachment.Title, sv.expectedEmoji)
		})
	}
}

func TestBuildProcessingAttachment(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{},
			Emoji:  map[string]string{},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	testAttachment := post.Attachment{
		Color:     "#CC0000",
		Title:     "🔴 Test Alert",
		TitleLink: "http://keep.ui/alerts/feed?fingerprint=fp-123",
		Fields: []post.AttachmentField{
			{Title: "Severity", Value: "CRITICAL", Short: true},
		},
	}
	attachmentJSON, err := testAttachment.ToJSON()
	require.NoError(t, err)

	tests := []struct {
		name          string
		action        string
		expectedStyle string
	}{
		{
			name:          "acknowledge action uses default style",
			action:        "acknowledge",
			expectedStyle: "default",
		},
		{
			name:          "resolve action uses success style",
			action:        "resolve",
			expectedStyle: "success",
		},
		{
			name:          "unacknowledge action uses default style",
			action:        "unacknowledge",
			expectedStyle: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attachment, err := builder.BuildProcessingAttachment(attachmentJSON, tt.action)
			require.NoError(t, err)

			assert.Equal(t, "#CC0000", attachment.Color, "should preserve original color")
			assert.Equal(t, "🔴 Test Alert", attachment.Title, "should preserve original title")
			assert.Contains(t, attachment.TitleLink, "fp-123", "should preserve title link")
			assert.Len(t, attachment.Fields, 1, "should preserve fields")
			assert.Len(t, attachment.Actions, 1, "should have exactly one button")
			assert.Equal(t, "processing", attachment.Actions[0].ID)
			assert.Equal(t, "Processing...", attachment.Actions[0].Name)
			assert.Equal(t, tt.expectedStyle, attachment.Actions[0].Style)
		})
	}
}

func TestBuildProcessingAttachment_InvalidJSON(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{},
			Emoji:  map[string]string{},
		},
		Labels: config.LabelsConfig{
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	_, err := builder.BuildProcessingAttachment("invalid json", "acknowledge")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deserialize attachment")
}

func TestBuildFiringAttachmentHasButtonStyles(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"high": "#FF6600"},
			Emoji:  map[string]string{"high": "🟠"},
		},
		Labels: config.LabelsConfig{
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, _ := alert.NewSeverity("high")
	fingerprint := alert.RestoreFingerprint("test-fp")
	status := alert.RestoreStatus(alert.StatusFiring)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Test Alert",
		severity,
		status,
		"",
		"prometheus",
		map[string]string{},
		time.Time{},
	)

	attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

	require.Len(t, attachment.Actions, 2)
	assert.Equal(t, "acknowledge", attachment.Actions[0].ID)
	assert.Equal(t, "default", attachment.Actions[0].Style, "acknowledge button should have default style")
	assert.Equal(t, "resolve", attachment.Actions[1].ID)
	assert.Equal(t, "success", attachment.Actions[1].Style, "resolve button should have success style")
}

func TestBuildAcknowledgedAttachmentHasButtonStyles(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"acknowledged": "#FFA500"},
			Emoji:  map[string]string{},
		},
		Labels: config.LabelsConfig{
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, _ := alert.NewSeverity("high")
	fingerprint := alert.RestoreFingerprint("test-fp")
	status := alert.RestoreStatus(alert.StatusAcknowledged)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Test Alert",
		severity,
		status,
		"",
		"prometheus",
		map[string]string{},
		time.Time{},
	)

	attachment := builder.BuildAcknowledgedAttachment(testAlert, "http://callback.url", "http://keep.ui", "testuser")

	require.Len(t, attachment.Actions, 2)
	assert.Equal(t, "unacknowledge", attachment.Actions[0].ID)
	assert.Equal(t, "default", attachment.Actions[0].Style, "unacknowledge button should have default style")
	assert.Equal(t, "resolve", attachment.Actions[1].ID)
	assert.Equal(t, "success", attachment.Actions[1].Style, "resolve button should have success style")
}

func boolPtr(b bool) *bool {
	return &b
}

func TestBuildFiringAttachment_SeverityFieldDisabled(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"critical": "#CC0000"},
			Emoji:  map[string]string{"critical": "🔴"},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
			Fields: config.FieldsConfig{
				ShowSeverity: boolPtr(false),
			},
		},
		Labels: config.LabelsConfig{
			Display:  []string{"host"},
			Exclude:  []string{},
			Rename:   map[string]string{},
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, err := alert.NewSeverity("critical")
	require.NoError(t, err)

	fingerprint := alert.RestoreFingerprint("test-fingerprint-123")
	status := alert.RestoreStatus(alert.StatusFiring)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"High CPU Usage",
		severity,
		status,
		"CPU usage exceeded 90%",
		"prometheus",
		map[string]string{"host": "server-1"},
		time.Time{},
	)

	attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

	for _, field := range attachment.Fields {
		assert.NotEqual(t, "Severity", field.Title, "Severity field should not appear when ShowSeverity is false")
	}

	var foundHost bool
	for _, field := range attachment.Fields {
		if field.Title == "host" {
			foundHost = true
			break
		}
	}
	assert.True(t, foundHost, "should still have display labels")
}

func TestBuildFiringAttachment_DescriptionFieldDisabled(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"warning": "#EDA200"},
			Emoji:  map[string]string{"warning": "🟡"},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
			Fields: config.FieldsConfig{
				ShowDescription: boolPtr(false),
			},
		},
		Labels: config.LabelsConfig{
			Display:  []string{"host"},
			Exclude:  []string{},
			Rename:   map[string]string{},
			Grouping: config.LabelGroupingConfig{},
		},
	}

	builder := NewBuilder(fileConfig)

	severity, err := alert.NewSeverity("warning")
	require.NoError(t, err)

	fingerprint := alert.RestoreFingerprint("test-fingerprint-456")
	status := alert.RestoreStatus(alert.StatusFiring)

	testAlert := alert.RestoreAlert(
		fingerprint,
		"Disk Space Low",
		severity,
		status,
		"Disk usage exceeded 85%",
		"prometheus",
		map[string]string{"host": "server-2"},
		time.Time{},
	)

	attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

	for _, field := range attachment.Fields {
		assert.NotEqual(t, "Description", field.Title, "Description field should not appear when ShowDescription is false")
	}

	var foundSeverity bool
	for _, field := range attachment.Fields {
		if field.Title == "Severity" {
			foundSeverity = true
			break
		}
	}
	assert.True(t, foundSeverity, "should still have Severity field")
}

func TestBuildFiringAttachment_SeverityPositions(t *testing.T) {
	tests := []struct {
		name             string
		severityPosition string
		displayLabels    []string
		labels           map[string]string
		expectedPosition string
	}{
		{
			name:             "first - Severity should be first field",
			severityPosition: "first",
			displayLabels:    []string{"host", "service"},
			labels:           map[string]string{"host": "server-1", "service": "api"},
			expectedPosition: "first",
		},
		{
			name:             "after_display - Severity should appear after display fields",
			severityPosition: "after_display",
			displayLabels:    []string{"host", "service"},
			labels:           map[string]string{"host": "server-1", "service": "api"},
			expectedPosition: "after_display",
		},
		{
			name:             "last - Severity should be last field",
			severityPosition: "last",
			displayLabels:    []string{"host", "service"},
			labels:           map[string]string{"host": "server-1", "service": "api"},
			expectedPosition: "last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileConfig := &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"high": "#FF6600"},
					Emoji:  map[string]string{"high": "🟠"},
					Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
					Fields: config.FieldsConfig{
						SeverityPosition: tt.severityPosition,
					},
				},
				Labels: config.LabelsConfig{
					Display:  tt.displayLabels,
					Exclude:  []string{},
					Rename:   map[string]string{},
					Grouping: config.LabelGroupingConfig{},
				},
			}

			builder := NewBuilder(fileConfig)

			severity, err := alert.NewSeverity("high")
			require.NoError(t, err)

			fingerprint := alert.RestoreFingerprint("test-fp-position")
			status := alert.RestoreStatus(alert.StatusFiring)

			testAlert := alert.RestoreAlert(
				fingerprint,
				"Test Alert",
				severity,
				status,
				"",
				"prometheus",
				tt.labels,
				time.Time{},
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			severityIndex := -1
			for i, field := range attachment.Fields {
				if field.Title == "Severity" {
					severityIndex = i
					break
				}
			}

			require.NotEqual(t, -1, severityIndex, "Severity field should exist")

			switch tt.expectedPosition {
			case "first":
				assert.Equal(t, 0, severityIndex, "Severity should be at index 0 for position 'first'")
			case "after_display":
				displayCount := 0
				for _, label := range tt.displayLabels {
					if _, ok := tt.labels[label]; ok {
						displayCount++
					}
				}
				assert.Equal(t, displayCount, severityIndex, "Severity should be at index %d (after display fields) for position 'after_display'", displayCount)
			case "last":
				assert.Equal(t, len(attachment.Fields)-1, severityIndex, "Severity should be at last index for position 'last'")
			}
		})
	}
}

func TestBuildFieldsWithGrouping(t *testing.T) {
	tests := []struct {
		name           string
		labels         map[string]string
		groupingConfig config.LabelGroupingConfig
		displayLabels  []string
		expectedGroups []string
		expectedLabels bool
	}{
		{
			name: "topology labels grouped when threshold met",
			labels: map[string]string{
				"topology_region": "us-east",
				"topology_zone":   "zone-a",
				"other_label":     "value",
			},
			groupingConfig: config.LabelGroupingConfig{
				Enabled:   true,
				Threshold: 2,
				Groups: []config.LabelGroupRule{
					{Prefixes: []string{"topology_"}, GroupName: "Topology", Priority: 100},
				},
			},
			displayLabels:  []string{},
			expectedGroups: []string{"Topology"},
			expectedLabels: true,
		},
		{
			name: "below threshold moves to ungrouped",
			labels: map[string]string{
				"topology_region": "us-east",
				"other_label":     "value",
			},
			groupingConfig: config.LabelGroupingConfig{
				Enabled:   true,
				Threshold: 2,
				Groups: []config.LabelGroupRule{
					{Prefixes: []string{"topology_"}, GroupName: "Topology", Priority: 100},
				},
			},
			displayLabels:  []string{},
			expectedGroups: []string{},
			expectedLabels: true,
		},
		{
			name: "multiple groups with priority ordering",
			labels: map[string]string{
				"topology_region":    "us-east",
				"topology_zone":      "zone-a",
				"kubernetes_io_node": "node-1",
				"kubernetes_io_zone": "k8s-zone",
			},
			groupingConfig: config.LabelGroupingConfig{
				Enabled:   true,
				Threshold: 2,
				Groups: []config.LabelGroupRule{
					{Prefixes: []string{"topology_"}, GroupName: "Topology", Priority: 100},
					{Prefixes: []string{"kubernetes_io_"}, GroupName: "Kubernetes", Priority: 90},
				},
			},
			displayLabels:  []string{},
			expectedGroups: []string{"Topology", "Kubernetes"},
			expectedLabels: false,
		},
		{
			name: "display labels not grouped",
			labels: map[string]string{
				"alertgroup":      "infrastructure",
				"topology_region": "us-east",
				"topology_zone":   "zone-a",
			},
			groupingConfig: config.LabelGroupingConfig{
				Enabled:   true,
				Threshold: 2,
				Groups: []config.LabelGroupRule{
					{Prefixes: []string{"topology_"}, GroupName: "Topology", Priority: 100},
				},
			},
			displayLabels:  []string{"alertgroup"},
			expectedGroups: []string{"Topology"},
			expectedLabels: false,
		},
		{
			name: "grouping disabled shows no groups",
			labels: map[string]string{
				"topology_region": "us-east",
				"topology_zone":   "zone-a",
			},
			groupingConfig: config.LabelGroupingConfig{
				Enabled: false,
			},
			displayLabels:  []string{},
			expectedGroups: []string{},
			expectedLabels: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileConfig := &config.FileConfig{
				Message: config.MessageConfig{
					Colors: map[string]string{"info": "#0066FF"},
					Emoji:  map[string]string{"info": "🔵"},
				},
				Labels: config.LabelsConfig{
					Display:  tt.displayLabels,
					Exclude:  []string{},
					Rename:   map[string]string{},
					Grouping: tt.groupingConfig,
				},
			}

			builder := NewBuilder(fileConfig)

			severity, _ := alert.NewSeverity("info")
			fingerprint := alert.RestoreFingerprint("test-fp")
			status := alert.RestoreStatus(alert.StatusFiring)

			testAlert := alert.RestoreAlert(
				fingerprint,
				"Test Alert",
				severity,
				status,
				"",
				"prometheus",
				tt.labels,
				time.Time{},
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			// Check expected groups exist
			for _, expectedGroup := range tt.expectedGroups {
				found := false
				for _, field := range attachment.Fields {
					if field.Title == expectedGroup {
						found = true
						break
					}
				}
				assert.True(t, found, "expected group %s not found", expectedGroup)
			}

			// Check Labels field
			hasLabelsField := false
			for _, field := range attachment.Fields {
				if field.Title == "Labels" {
					hasLabelsField = true
					break
				}
			}
			assert.Equal(t, tt.expectedLabels, hasLabelsField, "Labels field presence mismatch")

			// Verify group ordering by priority
			if len(tt.expectedGroups) > 1 {
				var groupIndices []int
				for _, expectedGroup := range tt.expectedGroups {
					for i, field := range attachment.Fields {
						if field.Title == expectedGroup {
							groupIndices = append(groupIndices, i)
							break
						}
					}
				}
				for i := 1; i < len(groupIndices); i++ {
					assert.Less(t, groupIndices[i-1], groupIndices[i], "groups should be ordered by priority")
				}
			}
		})
	}
}
