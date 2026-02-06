package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplyFileConfig(t *testing.T) {
	t.Run("env var set, file config set - env takes precedence", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "true")
		t.Setenv("POLLING_INTERVAL", "5m")
		t.Setenv("POLLING_ALERTS_LIMIT", "500")
		t.Setenv("KEEP_SETUP_ENABLED", "false")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled:     true,
				Interval:    5 * time.Minute,
				AlertsLimit: 500,
			},
			Setup: SetupConfig{
				Enabled: false,
			},
		}

		fileEnabled := true
		fileAlertsLimit := 2000
		fileSetupEnabled := true
		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Enabled:     &fileEnabled,
				Interval:    "10m",
				AlertsLimit: &fileAlertsLimit,
			},
			Setup: FileSetupConfig{
				Enabled: &fileSetupEnabled,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.True(t, cfg.Polling.Enabled, "env var should take precedence")
		assert.Equal(t, 5*time.Minute, cfg.Polling.Interval, "env var should take precedence")
		assert.Equal(t, 500, cfg.Polling.AlertsLimit, "env var should take precedence")
		assert.False(t, cfg.Setup.Enabled, "env var should take precedence")
	})

	t.Run("env var not set, file config set - file config applied", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "")
		t.Setenv("POLLING_INTERVAL", "")
		t.Setenv("POLLING_ALERTS_LIMIT", "")
		t.Setenv("KEEP_SETUP_ENABLED", "")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled:     false,
				Interval:    time.Minute,
				AlertsLimit: 1000,
			},
			Setup: SetupConfig{
				Enabled: true,
			},
		}

		fileEnabled := true
		fileAlertsLimit := 2000
		fileSetupEnabled := false
		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Enabled:     &fileEnabled,
				Interval:    "10m",
				AlertsLimit: &fileAlertsLimit,
			},
			Setup: FileSetupConfig{
				Enabled: &fileSetupEnabled,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.True(t, cfg.Polling.Enabled, "file config should be applied")
		assert.Equal(t, 10*time.Minute, cfg.Polling.Interval, "file config should be applied")
		assert.Equal(t, 2000, cfg.Polling.AlertsLimit, "file config should be applied")
		assert.False(t, cfg.Setup.Enabled, "file config should be applied")
	})

	t.Run("neither set - defaults remain", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "")
		t.Setenv("POLLING_INTERVAL", "")
		t.Setenv("POLLING_ALERTS_LIMIT", "")
		t.Setenv("KEEP_SETUP_ENABLED", "")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled:     false,
				Interval:    time.Minute,
				AlertsLimit: 1000,
			},
			Setup: SetupConfig{
				Enabled: true,
			},
		}

		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Enabled:     nil,
				Interval:    "",
				AlertsLimit: nil,
			},
			Setup: FileSetupConfig{
				Enabled: nil,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.False(t, cfg.Polling.Enabled, "defaults should remain")
		assert.Equal(t, time.Minute, cfg.Polling.Interval, "defaults should remain")
		assert.Equal(t, 1000, cfg.Polling.AlertsLimit, "defaults should remain")
		assert.True(t, cfg.Setup.Enabled, "defaults should remain")
	})

	t.Run("invalid duration string in file config - gracefully handled", func(t *testing.T) {
		t.Setenv("POLLING_INTERVAL", "")

		cfg := &Config{
			Polling: PollingConfig{
				Interval: 2 * time.Minute,
			},
		}

		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Interval: "invalid-duration",
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.Equal(t, 2*time.Minute, cfg.Polling.Interval, "original value should remain on parse error")
	})

	t.Run("partial file config - only specified fields applied", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "")
		t.Setenv("POLLING_INTERVAL", "")
		t.Setenv("POLLING_ALERTS_LIMIT", "")
		t.Setenv("KEEP_SETUP_ENABLED", "")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled:     false,
				Interval:    time.Minute,
				AlertsLimit: 1000,
			},
			Setup: SetupConfig{
				Enabled: true,
			},
		}

		fileEnabled := true
		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Enabled:     &fileEnabled,
				Interval:    "",
				AlertsLimit: nil,
			},
			Setup: FileSetupConfig{
				Enabled: nil,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.True(t, cfg.Polling.Enabled, "file config should be applied")
		assert.Equal(t, time.Minute, cfg.Polling.Interval, "original value should remain")
		assert.Equal(t, 1000, cfg.Polling.AlertsLimit, "original value should remain")
		assert.True(t, cfg.Setup.Enabled, "original value should remain")
	})

	t.Run("valid duration formats", func(t *testing.T) {
		t.Setenv("POLLING_INTERVAL", "")

		tests := []struct {
			name     string
			duration string
			expected time.Duration
		}{
			{"seconds", "30s", 30 * time.Second},
			{"minutes", "5m", 5 * time.Minute},
			{"hours", "2h", 2 * time.Hour},
			{"combined", "1h30m", 90 * time.Minute},
			{"milliseconds", "500ms", 500 * time.Millisecond},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &Config{
					Polling: PollingConfig{
						Interval: time.Minute,
					},
				}

				fileConfig := &FileConfig{
					Polling: FilePollingConfig{
						Interval: tt.duration,
					},
				}

				cfg.ApplyFileConfig(fileConfig)

				assert.Equal(t, tt.expected, cfg.Polling.Interval)
			})
		}
	})

	t.Run("file config with false values", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "")
		t.Setenv("KEEP_SETUP_ENABLED", "")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled: true,
			},
			Setup: SetupConfig{
				Enabled: true,
			},
		}

		filePollingEnabled := false
		fileSetupEnabled := false
		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Enabled: &filePollingEnabled,
			},
			Setup: FileSetupConfig{
				Enabled: &fileSetupEnabled,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.False(t, cfg.Polling.Enabled, "file config false should be applied")
		assert.False(t, cfg.Setup.Enabled, "file config false should be applied")
	})

	t.Run("file config with zero alerts limit", func(t *testing.T) {
		t.Setenv("POLLING_ALERTS_LIMIT", "")

		cfg := &Config{
			Polling: PollingConfig{
				AlertsLimit: 1000,
			},
		}

		fileAlertsLimit := 0
		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				AlertsLimit: &fileAlertsLimit,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.Equal(t, 0, cfg.Polling.AlertsLimit, "file config zero should be applied")
	})

	t.Run("empty file config", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "")
		t.Setenv("POLLING_INTERVAL", "")
		t.Setenv("POLLING_ALERTS_LIMIT", "")
		t.Setenv("KEEP_SETUP_ENABLED", "")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled:     false,
				Interval:    time.Minute,
				AlertsLimit: 1000,
			},
			Setup: SetupConfig{
				Enabled: true,
			},
		}

		fileConfig := &FileConfig{}

		cfg.ApplyFileConfig(fileConfig)

		assert.False(t, cfg.Polling.Enabled)
		assert.Equal(t, time.Minute, cfg.Polling.Interval)
		assert.Equal(t, 1000, cfg.Polling.AlertsLimit)
		assert.True(t, cfg.Setup.Enabled)
	})

	t.Run("env var explicitly set to empty string - file config not applied", func(t *testing.T) {
		t.Setenv("POLLING_ENABLED", "")
		t.Setenv("POLLING_INTERVAL", "")
		t.Setenv("POLLING_ALERTS_LIMIT", "")
		t.Setenv("KEEP_SETUP_ENABLED", "")

		cfg := &Config{
			Polling: PollingConfig{
				Enabled:     false,
				Interval:    time.Minute,
				AlertsLimit: 1000,
			},
			Setup: SetupConfig{
				Enabled: true,
			},
		}

		fileEnabled := true
		fileAlertsLimit := 2000
		fileSetupEnabled := false
		fileConfig := &FileConfig{
			Polling: FilePollingConfig{
				Enabled:     &fileEnabled,
				Interval:    "10m",
				AlertsLimit: &fileAlertsLimit,
			},
			Setup: FileSetupConfig{
				Enabled: &fileSetupEnabled,
			},
		}

		cfg.ApplyFileConfig(fileConfig)

		assert.True(t, cfg.Polling.Enabled, "file config should be applied when env is empty")
		assert.Equal(t, 10*time.Minute, cfg.Polling.Interval, "file config should be applied when env is empty")
		assert.Equal(t, 2000, cfg.Polling.AlertsLimit, "file config should be applied when env is empty")
		assert.False(t, cfg.Setup.Enabled, "file config should be applied when env is empty")
	})
}
