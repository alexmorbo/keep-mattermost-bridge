package alert

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fingerprint tests
func TestNewFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorIs     error
	}{
		{
			name:        "valid fingerprint",
			value:       "abc123",
			expectError: false,
		},
		{
			name:        "valid fingerprint with hyphen",
			value:       "fp-12345",
			expectError: false,
		},
		{
			name:        "valid fingerprint with dot and numbers",
			value:       "sha256.abcdef0123456789",
			expectError: false,
		},
		{
			name:        "valid fingerprint with underscore",
			value:       "alert_name.test-1",
			expectError: false,
		},
		{
			name:        "empty fingerprint returns error",
			value:       "",
			expectError: true,
			errorIs:     ErrInvalidFingerprint,
		},
		{
			name:        "fingerprint exceeds max length 512",
			value:       strings.Repeat("a", 513),
			expectError: true,
			errorIs:     ErrInvalidFingerprint,
		},
		{
			name:        "fingerprint with spaces",
			value:       "fp with spaces",
			expectError: true,
			errorIs:     ErrInvalidFingerprint,
		},
		{
			name:        "fingerprint with special characters",
			value:       "fp@special!",
			expectError: true,
			errorIs:     ErrInvalidFingerprint,
		},
		{
			name:        "fingerprint with newline",
			value:       "fp\nnewline",
			expectError: true,
			errorIs:     ErrInvalidFingerprint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp, err := NewFingerprint(tt.value)
			if tt.expectError {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.errorIs))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.value, fp.Value())
				assert.Equal(t, tt.value, fp.String())
			}
		})
	}
}

func TestRestoreFingerprint(t *testing.T) {
	value := "restored-fingerprint-123"
	fp := RestoreFingerprint(value)
	assert.Equal(t, value, fp.Value())
	assert.Equal(t, value, fp.String())
}

func TestFingerprintEquals(t *testing.T) {
	fp1 := RestoreFingerprint("fingerprint-123")
	fp2 := RestoreFingerprint("fingerprint-123")
	fp3 := RestoreFingerprint("fingerprint-456")

	assert.True(t, fp1.Equals(fp2))
	assert.True(t, fp2.Equals(fp1))
	assert.False(t, fp1.Equals(fp3))
	assert.False(t, fp3.Equals(fp1))
}

// Severity tests
func TestNewSeverity(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expected    string
		expectError bool
	}{
		{
			name:        "critical severity",
			value:       SeverityCritical,
			expected:    SeverityCritical,
			expectError: false,
		},
		{
			name:        "critical severity uppercase",
			value:       "CRITICAL",
			expected:    SeverityCritical,
			expectError: false,
		},
		{
			name:        "high severity",
			value:       SeverityHigh,
			expected:    SeverityHigh,
			expectError: false,
		},
		{
			name:        "high severity mixed case",
			value:       "High",
			expected:    SeverityHigh,
			expectError: false,
		},
		{
			name:        "warning severity",
			value:       SeverityWarning,
			expected:    SeverityWarning,
			expectError: false,
		},
		{
			name:        "info severity",
			value:       SeverityInfo,
			expected:    SeverityInfo,
			expectError: false,
		},
		{
			name:        "invalid severity",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "empty severity",
			value:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity, err := NewSeverity(tt.value)
			if tt.expectError {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidSeverity))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, severity.Value())
				assert.Equal(t, tt.expected, severity.String())
			}
		})
	}
}

func TestRestoreSeverity(t *testing.T) {
	value := "critical"
	severity := RestoreSeverity(value)
	assert.Equal(t, value, severity.Value())
	assert.Equal(t, value, severity.String())
}

func TestSeverityIsCritical(t *testing.T) {
	critical := RestoreSeverity(SeverityCritical)
	high := RestoreSeverity(SeverityHigh)
	warning := RestoreSeverity(SeverityWarning)
	info := RestoreSeverity(SeverityInfo)

	assert.True(t, critical.IsCritical())
	assert.False(t, high.IsCritical())
	assert.False(t, warning.IsCritical())
	assert.False(t, info.IsCritical())
}

func TestSeverityIsHigh(t *testing.T) {
	critical := RestoreSeverity(SeverityCritical)
	high := RestoreSeverity(SeverityHigh)
	warning := RestoreSeverity(SeverityWarning)
	info := RestoreSeverity(SeverityInfo)

	assert.False(t, critical.IsHigh())
	assert.True(t, high.IsHigh())
	assert.False(t, warning.IsHigh())
	assert.False(t, info.IsHigh())
}

func TestSeverityIsWarning(t *testing.T) {
	critical := RestoreSeverity(SeverityCritical)
	high := RestoreSeverity(SeverityHigh)
	warning := RestoreSeverity(SeverityWarning)
	info := RestoreSeverity(SeverityInfo)

	assert.False(t, critical.IsWarning())
	assert.False(t, high.IsWarning())
	assert.True(t, warning.IsWarning())
	assert.False(t, info.IsWarning())
}

func TestSeverityIsInfo(t *testing.T) {
	critical := RestoreSeverity(SeverityCritical)
	high := RestoreSeverity(SeverityHigh)
	warning := RestoreSeverity(SeverityWarning)
	info := RestoreSeverity(SeverityInfo)

	assert.False(t, critical.IsInfo())
	assert.False(t, high.IsInfo())
	assert.False(t, warning.IsInfo())
	assert.True(t, info.IsInfo())
}

// Status tests
func TestNewStatus(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expected    string
		expectError bool
	}{
		{
			name:        "firing status",
			value:       StatusFiring,
			expected:    StatusFiring,
			expectError: false,
		},
		{
			name:        "firing status uppercase",
			value:       "FIRING",
			expected:    StatusFiring,
			expectError: false,
		},
		{
			name:        "resolved status",
			value:       StatusResolved,
			expected:    StatusResolved,
			expectError: false,
		},
		{
			name:        "resolved status mixed case",
			value:       "Resolved",
			expected:    StatusResolved,
			expectError: false,
		},
		{
			name:        "acknowledged status",
			value:       StatusAcknowledged,
			expected:    StatusAcknowledged,
			expectError: false,
		},
		{
			name:        "invalid status",
			value:       "invalid",
			expectError: true,
		},
		{
			name:        "empty status",
			value:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := NewStatus(tt.value)
			if tt.expectError {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidStatus))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, status.Value())
				assert.Equal(t, tt.expected, status.String())
			}
		})
	}
}

func TestRestoreStatus(t *testing.T) {
	value := "firing"
	status := RestoreStatus(value)
	assert.Equal(t, value, status.Value())
	assert.Equal(t, value, status.String())
}

func TestStatusIsFiring(t *testing.T) {
	firing := RestoreStatus(StatusFiring)
	resolved := RestoreStatus(StatusResolved)
	acknowledged := RestoreStatus(StatusAcknowledged)

	assert.True(t, firing.IsFiring())
	assert.False(t, resolved.IsFiring())
	assert.False(t, acknowledged.IsFiring())
}

func TestStatusIsResolved(t *testing.T) {
	firing := RestoreStatus(StatusFiring)
	resolved := RestoreStatus(StatusResolved)
	acknowledged := RestoreStatus(StatusAcknowledged)

	assert.False(t, firing.IsResolved())
	assert.True(t, resolved.IsResolved())
	assert.False(t, acknowledged.IsResolved())
}

func TestStatusIsAcknowledged(t *testing.T) {
	firing := RestoreStatus(StatusFiring)
	resolved := RestoreStatus(StatusResolved)
	acknowledged := RestoreStatus(StatusAcknowledged)

	assert.False(t, firing.IsAcknowledged())
	assert.False(t, resolved.IsAcknowledged())
	assert.True(t, acknowledged.IsAcknowledged())
}

// Alert entity tests
func TestNewAlert(t *testing.T) {
	validFingerprint := RestoreFingerprint("fp-123")
	validSeverity := RestoreSeverity(SeverityCritical)
	validStatus := RestoreStatus(StatusFiring)

	t.Run("create valid alert with all fields", func(t *testing.T) {
		labels := map[string]string{
			"env":     "production",
			"service": "api",
		}

		alert, err := NewAlert(
			validFingerprint,
			"Test Alert",
			validSeverity,
			validStatus,
			"Test description",
			"prometheus",
			labels,
		)

		require.NoError(t, err)
		require.NotNil(t, alert)
		assert.True(t, alert.Fingerprint().Equals(validFingerprint))
		assert.Equal(t, "Test Alert", alert.Name())
		assert.Equal(t, validSeverity, alert.Severity())
		assert.Equal(t, validStatus, alert.Status())
		assert.Equal(t, "Test description", alert.Description())
		assert.Equal(t, "prometheus", alert.Source())

		resultLabels := alert.Labels()
		assert.Equal(t, 2, len(resultLabels))
		assert.Equal(t, "production", resultLabels["env"])
		assert.Equal(t, "api", resultLabels["service"])
	})

	t.Run("create alert with nil labels initializes empty map", func(t *testing.T) {
		alert, err := NewAlert(
			validFingerprint,
			"Test Alert",
			validSeverity,
			validStatus,
			"Description",
			"source",
			nil,
		)

		require.NoError(t, err)
		labels := alert.Labels()
		assert.NotNil(t, labels)
		assert.Equal(t, 0, len(labels))
	})

	t.Run("create alert with empty name returns error", func(t *testing.T) {
		alert, err := NewAlert(
			validFingerprint,
			"",
			validSeverity,
			validStatus,
			"Description",
			"source",
			nil,
		)

		require.Error(t, err)
		assert.Nil(t, alert)
		assert.True(t, errors.Is(err, ErrInvalidAlert))
	})

	t.Run("labels are copied and not shared", func(t *testing.T) {
		originalLabels := map[string]string{
			"key": "value",
		}

		alert, err := NewAlert(
			validFingerprint,
			"Test Alert",
			validSeverity,
			validStatus,
			"Description",
			"source",
			originalLabels,
		)

		require.NoError(t, err)

		// Modify returned labels
		returnedLabels := alert.Labels()
		returnedLabels["new_key"] = "new_value"

		// Original labels should be unchanged
		assert.Equal(t, 1, len(alert.Labels()))
		assert.Equal(t, "value", alert.Labels()["key"])
		assert.Empty(t, alert.Labels()["new_key"])
	})
}

func TestRestoreAlert(t *testing.T) {
	fingerprint := RestoreFingerprint("fp-456")
	severity := RestoreSeverity(SeverityHigh)
	status := RestoreStatus(StatusResolved)
	labels := map[string]string{
		"region": "us-east-1",
	}

	t.Run("restore alert with all fields", func(t *testing.T) {
		alert := RestoreAlert(
			fingerprint,
			"Restored Alert",
			severity,
			status,
			"Restored description",
			"alertmanager",
			labels,
		)

		require.NotNil(t, alert)
		assert.True(t, alert.Fingerprint().Equals(fingerprint))
		assert.Equal(t, "Restored Alert", alert.Name())
		assert.Equal(t, severity, alert.Severity())
		assert.Equal(t, status, alert.Status())
		assert.Equal(t, "Restored description", alert.Description())
		assert.Equal(t, "alertmanager", alert.Source())
		assert.Equal(t, "us-east-1", alert.Labels()["region"])
	})

	t.Run("restore alert with nil labels initializes empty map", func(t *testing.T) {
		alert := RestoreAlert(
			fingerprint,
			"Alert",
			severity,
			status,
			"Description",
			"source",
			nil,
		)

		labels := alert.Labels()
		assert.NotNil(t, labels)
		assert.Equal(t, 0, len(labels))
	})
}

func TestAlertGetters(t *testing.T) {
	fingerprint := RestoreFingerprint("fp-789")
	severity := RestoreSeverity(SeverityWarning)
	status := RestoreStatus(StatusAcknowledged)
	labels := map[string]string{
		"team": "platform",
		"app":  "database",
	}

	alert := RestoreAlert(
		fingerprint,
		"Database Alert",
		severity,
		status,
		"High connection count",
		"custom-monitor",
		labels,
	)

	t.Run("fingerprint getter", func(t *testing.T) {
		assert.True(t, alert.Fingerprint().Equals(fingerprint))
	})

	t.Run("name getter", func(t *testing.T) {
		assert.Equal(t, "Database Alert", alert.Name())
	})

	t.Run("severity getter", func(t *testing.T) {
		assert.Equal(t, severity, alert.Severity())
		assert.True(t, alert.Severity().IsWarning())
	})

	t.Run("status getter", func(t *testing.T) {
		assert.Equal(t, status, alert.Status())
		assert.True(t, alert.Status().IsAcknowledged())
	})

	t.Run("description getter", func(t *testing.T) {
		assert.Equal(t, "High connection count", alert.Description())
	})

	t.Run("source getter", func(t *testing.T) {
		assert.Equal(t, "custom-monitor", alert.Source())
	})

	t.Run("labels getter returns copy", func(t *testing.T) {
		labels1 := alert.Labels()
		labels2 := alert.Labels()

		assert.Equal(t, labels1, labels2)
		assert.Equal(t, "platform", labels1["team"])
		assert.Equal(t, "database", labels1["app"])

		// Modifying one copy shouldn't affect the other
		labels1["new"] = "value"
		assert.Empty(t, labels2["new"])
		assert.Empty(t, alert.Labels()["new"])
	})
}
