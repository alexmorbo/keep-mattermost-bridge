package messagebuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexmorbo/keep-mattermost-bridge/domain/alert"
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
					Display: []string{"host", "job"},
					Exclude: []string{},
					Rename:  map[string]string{"host": "Server"},
				},
			},
			alertSeverity:  "critical",
			alertName:      "High CPU Usage",
			alertDesc:      "CPU usage exceeded 90%",
			labels:         map[string]string{"host": "server-1", "job": "monitoring"},
			expectedColor:  "#CC0000",
			expectedEmoji:  "🔴",
			expectedFields: 3,
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
					Display: []string{"service"},
					Exclude: []string{},
					Rename:  map[string]string{},
				},
			},
			alertSeverity:  "warning",
			alertName:      "Disk Space Low",
			alertDesc:      "",
			labels:         map[string]string{"service": "api", "ignored": "value"},
			expectedColor:  "#EDA200",
			expectedEmoji:  "🟡",
			expectedFields: 1,
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
					Display: []string{},
					Exclude: []string{"internal"},
					Rename:  map[string]string{},
				},
			},
			alertSeverity:  "info",
			alertName:      "Service Started",
			alertDesc:      "Service started successfully",
			labels:         map[string]string{"host": "server-1", "internal": "skip-me"},
			expectedColor:  "#0066FF",
			expectedEmoji:  "🔵",
			expectedFields: 2,
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
					Display: []string{},
					Exclude: []string{},
					Rename:  map[string]string{"job": "Job Name", "host": "Hostname"},
				},
			},
			alertSeverity:  "high",
			alertName:      "Database Connection Failed",
			alertDesc:      "Cannot connect to database",
			labels:         map[string]string{"job": "db-monitor", "host": "db-01"},
			expectedColor:  "#FF6600",
			expectedEmoji:  "🟠",
			expectedFields: 3,
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
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			assert.Equal(t, tt.expectedColor, attachment.Color, "color mismatch")
			assert.Contains(t, attachment.Title, tt.expectedEmoji, "emoji not in title")
			assert.Contains(t, attachment.Title, tt.alertName, "alert name not in title")
			assert.Contains(t, attachment.TitleLink, "http://keep.ui/alerts/feed?fingerprint=test-fingerprint-123")
			assert.Equal(t, tt.expectedFields, len(attachment.Fields), "fields count mismatch")

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
			Display: []string{},
			Exclude: []string{},
			Rename:  map[string]string{},
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
	)

	attachment := builder.BuildAcknowledgedAttachment(testAlert, "http://callback.url", "http://keep.ui", "john.doe")

	assert.Equal(t, "#FFA500", attachment.Color, "should have orange color")
	assert.Contains(t, attachment.Title, "👀")
	assert.Contains(t, attachment.Title, "ACKNOWLEDGED")
	assert.Contains(t, attachment.Title, "Test Alert")
	assert.Contains(t, attachment.TitleLink, "http://keep.ui/alerts/feed?fingerprint=ack-fingerprint-456")

	assert.Len(t, attachment.Actions, 1, "should have only Resolve button")
	assert.Equal(t, "resolve", attachment.Actions[0].ID)
	assert.Equal(t, "Resolve", attachment.Actions[0].Name)
}

func TestBuildResolvedAttachment(t *testing.T) {
	fileConfig := &config.FileConfig{
		Message: config.MessageConfig{
			Colors: map[string]string{"resolved": "#00CC00"},
			Emoji:  map[string]string{},
			Footer: config.FooterConfig{Text: "Keep AIOps", IconURL: "https://test.com/icon.png"},
		},
		Labels: config.LabelsConfig{
			Display: []string{},
			Exclude: []string{},
			Rename:  map[string]string{},
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
	)

	attachment := builder.BuildResolvedAttachment(testAlert, "http://keep.ui")

	assert.Equal(t, "#00CC00", attachment.Color, "should have green color")
	assert.Contains(t, attachment.Title, "✅")
	assert.Contains(t, attachment.Title, "RESOLVED")
	assert.Contains(t, attachment.Title, "Resolved Alert")
	assert.Contains(t, attachment.TitleLink, "http://keep.ui/alerts/feed?fingerprint=resolved-fingerprint-789")

	assert.Len(t, attachment.Actions, 0, "should have no buttons")
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
			expectedCount: 2,
			checkTitles:   []string{"host", "env"},
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
			expectedCount: 2,
			checkTitles:   []string{"host", "service"},
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
			expectedCount: 2,
			checkTitles:   []string{"Server Name", "Environment"},
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
			expectedCount: 2,
			checkTitles:   []string{"host", "env"},
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
					Display: tt.displayLabels,
					Exclude: tt.excludeLabels,
					Rename:  tt.renameLabels,
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
		Labels: config.LabelsConfig{},
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
			)

			attachment := builder.BuildFiringAttachment(testAlert, "http://callback.url", "http://keep.ui")

			assert.Equal(t, sv.expectedColor, attachment.Color)
			assert.Contains(t, attachment.Title, sv.expectedEmoji)
		})
	}
}
