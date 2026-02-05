package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromFileValid(t *testing.T) {
	yamlContent := `
channels:
  default_channel_id: "general"
  routing:
    - severity: "critical"
      channel_id: "critical-alerts"
    - severity: "high"
      channel_id: "high-alerts"

message:
  colors:
    critical: "#FF0000"
    high: "#FFA500"
    warning: "#FFFF00"
    info: "#00FF00"
  emoji:
    critical: "🚨"
    high: "🔥"
  footer:
    text: "Custom Footer"
    icon_url: "https://custom.com/icon.png"
  bot:
    username: "CustomBot"
    icon_url: "https://custom.com/bot.png"

labels:
  display:
    - "host"
    - "service"
  exclude:
    - "internal"
    - "debug"
  rename:
    host: "Server"
    service: "Service Name"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "general", cfg.Channels.DefaultChannelID)
	assert.Len(t, cfg.Channels.Routing, 2)
	assert.Equal(t, "critical", cfg.Channels.Routing[0].Severity)
	assert.Equal(t, "critical-alerts", cfg.Channels.Routing[0].ChannelID)

	assert.Equal(t, "#FF0000", cfg.Message.Colors["critical"])
	assert.Equal(t, "🚨", cfg.Message.Emoji["critical"])
	assert.Equal(t, "Custom Footer", cfg.Message.Footer.Text)
	assert.Equal(t, "https://custom.com/icon.png", cfg.Message.Footer.IconURL)

	assert.Contains(t, cfg.Labels.Display, "host")
	assert.Contains(t, cfg.Labels.Display, "service")
	assert.Contains(t, cfg.Labels.Exclude, "internal")
	assert.Equal(t, "Server", cfg.Labels.Rename["host"])
}

func TestLoadFromFileNonExistent(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/config.yaml")
	require.NoError(t, err, "should return defaults for non-existent file")
	require.NotNil(t, cfg)

	assert.Equal(t, "", cfg.Channels.DefaultChannelID)
	assert.NotNil(t, cfg.Message.Colors)
	assert.NotNil(t, cfg.Message.Emoji)
	assert.Equal(t, "Keep AIOps", cfg.Message.Footer.Text)
}

func TestLoadFromFileInvalidYAML(t *testing.T) {
	invalidYAML := `
channels:
  default: "general"
  routing:
    - severity: "critical
      channel: "critical-alerts"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte(invalidYAML), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	assert.Error(t, err, "should return error for invalid YAML")
	assert.Nil(t, cfg)
}

func TestChannelForSeverity(t *testing.T) {
	cfg := &FileConfig{
		Channels: ChannelsConfig{
			DefaultChannelID: "default-channel",
			Routing: []RoutingRule{
				{Severity: "critical", ChannelID: "critical-alerts"},
				{Severity: "high", ChannelID: "high-alerts"},
				{Severity: "warning", ChannelID: "warning-alerts"},
			},
		},
	}

	tests := []struct {
		severity        string
		expectedChannel string
	}{
		{"critical", "critical-alerts"},
		{"high", "high-alerts"},
		{"warning", "warning-alerts"},
		{"info", "default-channel"},
		{"unknown", "default-channel"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			channel := cfg.ChannelIDForSeverity(tt.severity)
			assert.Equal(t, tt.expectedChannel, channel)
		})
	}
}

func TestColorForSeverity(t *testing.T) {
	cfg := &FileConfig{
		Message: MessageConfig{
			Colors: map[string]string{
				"critical": "#CC0000",
				"high":     "#FF6600",
				"warning":  "#EDA200",
				"info":     "#0066FF",
			},
		},
	}

	tests := []struct {
		severity      string
		expectedColor string
	}{
		{"critical", "#CC0000"},
		{"high", "#FF6600"},
		{"warning", "#EDA200"},
		{"info", "#0066FF"},
		{"unknown", "#808080"},
		{"", "#808080"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			color := cfg.ColorForSeverity(tt.severity)
			assert.Equal(t, tt.expectedColor, color)
		})
	}
}

func TestEmojiForSeverity(t *testing.T) {
	cfg := &FileConfig{
		Message: MessageConfig{
			Emoji: map[string]string{
				"critical": "🔴",
				"high":     "🟠",
				"warning":  "🟡",
				"info":     "🔵",
			},
		},
	}

	tests := []struct {
		severity      string
		expectedEmoji string
	}{
		{"critical", "🔴"},
		{"high", "🟠"},
		{"warning", "🟡"},
		{"info", "🔵"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			emoji := cfg.EmojiForSeverity(tt.severity)
			assert.Equal(t, tt.expectedEmoji, emoji)
		})
	}
}

func TestIsLabelExcluded(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"internal", "debug", "temp"},
			},
		}

		tests := []struct {
			label    string
			excluded bool
		}{
			{"internal", true},
			{"debug", true},
			{"temp", true},
			{"host", false},
			{"service", false},
			{"", false},
		}

		for _, tt := range tests {
			t.Run(tt.label, func(t *testing.T) {
				result := cfg.IsLabelExcluded(tt.label)
				assert.Equal(t, tt.excluded, result)
			})
		}
	})

	t.Run("wildcard prefix", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"talos_*"},
			},
		}

		tests := []struct {
			label    string
			excluded bool
		}{
			{"talos_version", true},
			{"talos_", true},
			{"talos", false},
			{"other_label", false},
		}

		for _, tt := range tests {
			t.Run(tt.label, func(t *testing.T) {
				result := cfg.IsLabelExcluded(tt.label)
				assert.Equal(t, tt.excluded, result)
			})
		}
	})

	t.Run("internal labels wildcard", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"__*"},
			},
		}

		tests := []struct {
			label    string
			excluded bool
		}{
			{"__name__", true},
			{"__address__", true},
			{"__", true},
			{"_single", false},
			{"normal", false},
		}

		for _, tt := range tests {
			t.Run(tt.label, func(t *testing.T) {
				result := cfg.IsLabelExcluded(tt.label)
				assert.Equal(t, tt.excluded, result)
			})
		}
	})

	t.Run("mixed patterns", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"prometheus", "__*", "job", "talos_*"},
			},
		}

		tests := []struct {
			label    string
			excluded bool
		}{
			{"prometheus", true},
			{"__name__", true},
			{"job", true},
			{"talos_version", true},
			{"alertname", false},
			{"instance", false},
		}

		for _, tt := range tests {
			t.Run(tt.label, func(t *testing.T) {
				result := cfg.IsLabelExcluded(tt.label)
				assert.Equal(t, tt.excluded, result)
			})
		}
	})

	t.Run("empty exclude list", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{},
			},
		}

		assert.False(t, cfg.IsLabelExcluded("any_label"))
		assert.False(t, cfg.IsLabelExcluded("__name__"))
	})

	t.Run("single asterisk wildcard matches everything", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"*"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("any_label"))
		assert.True(t, cfg.IsLabelExcluded("__name__"))
		assert.True(t, cfg.IsLabelExcluded(""))
	})

	t.Run("asterisk in middle matches glob", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"foo*bar"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("foo*bar"))
		assert.True(t, cfg.IsLabelExcluded("foobar"))
		assert.True(t, cfg.IsLabelExcluded("fooxbar"))
		assert.True(t, cfg.IsLabelExcluded("foo123bar"))
		assert.False(t, cfg.IsLabelExcluded("foobarbaz"))
	})

	t.Run("wildcard suffix pattern", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"*_kubernetes_io_zone"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("topology_kubernetes_io_zone"))
		assert.True(t, cfg.IsLabelExcluded("failure_domain_beta_kubernetes_io_zone"))
		assert.True(t, cfg.IsLabelExcluded("_kubernetes_io_zone"))
		assert.False(t, cfg.IsLabelExcluded("kubernetes_io_zone_extra"))
	})

	t.Run("wildcard both sides", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"*kubernetes*"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("kubernetes"))
		assert.True(t, cfg.IsLabelExcluded("beta_kubernetes_io"))
		assert.True(t, cfg.IsLabelExcluded("my_kubernetes_label"))
		assert.False(t, cfg.IsLabelExcluded("k8s_label"))
	})

	t.Run("question mark matches single character", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"label?"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("label1"))
		assert.True(t, cfg.IsLabelExcluded("labelx"))
		assert.False(t, cfg.IsLabelExcluded("label"))
		assert.False(t, cfg.IsLabelExcluded("label12"))
	})

	t.Run("character class pattern", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"node[0-9]"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("node0"))
		assert.True(t, cfg.IsLabelExcluded("node5"))
		assert.True(t, cfg.IsLabelExcluded("node9"))
		assert.False(t, cfg.IsLabelExcluded("nodex"))
		assert.False(t, cfg.IsLabelExcluded("node10"))
	})

	t.Run("negated character class", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"[^_]*"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("normal_label"))
		assert.True(t, cfg.IsLabelExcluded("alertname"))
		assert.False(t, cfg.IsLabelExcluded("_internal"))
	})

	t.Run("escaped special characters", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"literal\\*star"},
			},
		}

		assert.True(t, cfg.IsLabelExcluded("literal*star"))
		assert.False(t, cfg.IsLabelExcluded("literalxstar"))
	})

	t.Run("nil exclude list", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: nil,
			},
		}

		assert.False(t, cfg.IsLabelExcluded("any_label"))
		assert.False(t, cfg.IsLabelExcluded("__name__"))
	})
}

func TestIsLabelDisplayed(t *testing.T) {
	t.Run("with display list", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Display: []string{"host", "service", "env"},
			},
		}

		tests := []struct {
			label     string
			displayed bool
		}{
			{"host", true},
			{"service", true},
			{"env", true},
			{"region", false},
			{"zone", false},
			{"", false},
		}

		for _, tt := range tests {
			t.Run(tt.label, func(t *testing.T) {
				result := cfg.IsLabelDisplayed(tt.label)
				assert.Equal(t, tt.displayed, result)
			})
		}
	})

	t.Run("with empty display list", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Display: []string{},
			},
		}

		assert.True(t, cfg.IsLabelDisplayed("host"))
		assert.True(t, cfg.IsLabelDisplayed("service"))
		assert.True(t, cfg.IsLabelDisplayed("any-label"))
	})
}

func TestRenameLabel(t *testing.T) {
	cfg := &FileConfig{
		Labels: LabelsConfig{
			Rename: map[string]string{
				"host":    "Server Name",
				"service": "Service",
				"env":     "Environment",
			},
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"host", "Server Name"},
		{"service", "Service"},
		{"env", "Environment"},
		{"region", "region"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cfg.RenameLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &FileConfig{}
	cfg.applyDefaults()

	assert.Equal(t, "", cfg.Channels.DefaultChannelID)

	assert.NotNil(t, cfg.Message.Colors)
	assert.Equal(t, "#CC0000", cfg.Message.Colors["critical"])
	assert.Equal(t, "#FF6600", cfg.Message.Colors["high"])
	assert.Equal(t, "#EDA200", cfg.Message.Colors["warning"])
	assert.Equal(t, "#0066FF", cfg.Message.Colors["info"])
	assert.Equal(t, "#FFA500", cfg.Message.Colors["acknowledged"])
	assert.Equal(t, "#00CC00", cfg.Message.Colors["resolved"])

	assert.NotNil(t, cfg.Message.Emoji)
	assert.Equal(t, "🔴", cfg.Message.Emoji["critical"])
	assert.Equal(t, "🟠", cfg.Message.Emoji["high"])
	assert.Equal(t, "🟡", cfg.Message.Emoji["warning"])
	assert.Equal(t, "🔵", cfg.Message.Emoji["info"])

	assert.Equal(t, "Keep AIOps", cfg.Message.Footer.Text)
	assert.Equal(t, "https://avatars.githubusercontent.com/u/109032290?v=4", cfg.Message.Footer.IconURL)

	assert.NotNil(t, cfg.Labels.Rename)

	assert.NotNil(t, cfg.Users.Mapping)
}

func TestDefaultFileConfig(t *testing.T) {
	cfg := defaultFileConfig()

	require.NotNil(t, cfg)
	assert.Equal(t, "", cfg.Channels.DefaultChannelID)
	assert.NotNil(t, cfg.Message.Colors)
	assert.NotNil(t, cfg.Message.Emoji)
	assert.Equal(t, "Keep AIOps", cfg.Message.Footer.Text)
}

func TestPartialConfig(t *testing.T) {
	yamlContent := `
channels:
  default_channel_id: "custom-channel"

message:
  footer:
    text: "Custom Footer"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "custom-channel", cfg.Channels.DefaultChannelID)
	assert.Equal(t, "Custom Footer", cfg.Message.Footer.Text)

	assert.NotNil(t, cfg.Message.Colors, "should have default colors map")
	assert.Equal(t, "#CC0000", cfg.Message.Colors["critical"], "should have default critical color")
}

func TestLoadFromEnvValid(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token-123")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-api-key-456")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("REDIS_ADDR", "redis.example.com:6379")
	t.Setenv("REDIS_PASSWORD", "redis-password")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("CONFIG_PATH", "/custom/config.yaml")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "debug", cfg.Server.LogLevel)
	assert.Equal(t, "https://mattermost.example.com", cfg.Mattermost.URL)
	assert.Equal(t, "test-token-123", cfg.Mattermost.Token)
	assert.Equal(t, "https://keep.example.com", cfg.Keep.URL)
	assert.Equal(t, "keep-api-key-456", cfg.Keep.APIKey)
	assert.Equal(t, "https://keep-ui.example.com", cfg.Keep.UIURL)
	assert.Equal(t, "redis.example.com:6379", cfg.Redis.Addr)
	assert.Equal(t, "redis-password", cfg.Redis.Password)
	assert.Equal(t, 2, cfg.Redis.DB)
	assert.Equal(t, "/custom/config.yaml", cfg.ConfigPath)
	assert.Equal(t, "https://callback.example.com", cfg.CallbackURL)
}

func TestLoadFromEnvDefaults(t *testing.T) {
	// Required vars
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	// Clear env vars that have defaults to ensure isolation from CI
	t.Setenv("SERVER_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("CONFIG_PATH", "")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 8080, cfg.Server.Port, "should use default port")
	assert.Equal(t, "info", cfg.Server.LogLevel, "should use default log level")
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr, "should use default redis addr")
	assert.Equal(t, "", cfg.Redis.Password, "should have empty redis password by default")
	assert.Equal(t, 0, cfg.Redis.DB, "should use default redis db")
	assert.Equal(t, "/etc/kmbridge/config.yaml", cfg.ConfigPath, "should use default config path")
}

func TestLoadFromEnvMissingMattermostURL(t *testing.T) {
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "MATTERMOST_URL")
}

func TestLoadFromEnvMissingMattermostToken(t *testing.T) {
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "MATTERMOST_TOKEN")
}

func TestLoadFromEnvMissingKeepURL(t *testing.T) {
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "KEEP_URL")
}

func TestLoadFromEnvMissingKeepAPIKey(t *testing.T) {
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "KEEP_API_KEY")
}

func TestLoadFromEnvMissingKeepUIURL(t *testing.T) {
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "KEEP_UI_URL")
}

func TestLoadFromEnvInvalidPort(t *testing.T) {
	t.Setenv("SERVER_PORT", "invalid")
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "SERVER_PORT")
}

func TestLoadFromEnvInvalidRedisDB(t *testing.T) {
	t.Setenv("REDIS_DB", "not-a-number")
	t.Setenv("MATTERMOST_URL", "https://mattermost.example.com")
	t.Setenv("MATTERMOST_TOKEN", "test-token")
	t.Setenv("KEEP_URL", "https://keep.example.com")
	t.Setenv("KEEP_API_KEY", "keep-key")
	t.Setenv("KEEP_UI_URL", "https://keep-ui.example.com")
	t.Setenv("CALLBACK_URL", "https://callback.example.com")

	cfg, err := LoadFromEnv()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "REDIS_DB")
}

func TestFooterText(t *testing.T) {
	t.Run("returns configured footer text", func(t *testing.T) {
		cfg := &FileConfig{
			Message: MessageConfig{
				Footer: FooterConfig{
					Text: "Custom Footer Text",
				},
			},
		}

		assert.Equal(t, "Custom Footer Text", cfg.FooterText())
	})

	t.Run("returns default footer text after applyDefaults", func(t *testing.T) {
		cfg := &FileConfig{}
		cfg.applyDefaults()

		assert.Equal(t, "Keep AIOps", cfg.FooterText())
	})

	t.Run("returns empty string when not configured", func(t *testing.T) {
		cfg := &FileConfig{}

		assert.Equal(t, "", cfg.FooterText())
	})
}

func TestFooterIconURL(t *testing.T) {
	t.Run("returns configured footer icon URL", func(t *testing.T) {
		cfg := &FileConfig{
			Message: MessageConfig{
				Footer: FooterConfig{
					IconURL: "https://custom.example.com/icon.png",
				},
			},
		}

		assert.Equal(t, "https://custom.example.com/icon.png", cfg.FooterIconURL())
	})

	t.Run("returns default footer icon URL after applyDefaults", func(t *testing.T) {
		cfg := &FileConfig{}
		cfg.applyDefaults()

		assert.Equal(t, "https://avatars.githubusercontent.com/u/109032290?v=4", cfg.FooterIconURL())
	})

	t.Run("returns empty string when not configured", func(t *testing.T) {
		cfg := &FileConfig{}

		assert.Equal(t, "", cfg.FooterIconURL())
	})
}

func TestGetKeepUsername(t *testing.T) {
	t.Run("returns mapped Keep username", func(t *testing.T) {
		cfg := &FileConfig{
			Users: UsersConfig{
				Mapping: map[string]string{
					"alexmorbo":    "alex.keep",
					"another_user": "another.keep",
				},
			},
		}

		keepUser, ok := cfg.GetKeepUsername("alexmorbo")
		assert.True(t, ok)
		assert.Equal(t, "alex.keep", keepUser)

		keepUser, ok = cfg.GetKeepUsername("another_user")
		assert.True(t, ok)
		assert.Equal(t, "another.keep", keepUser)
	})

	t.Run("returns false for unmapped user", func(t *testing.T) {
		cfg := &FileConfig{
			Users: UsersConfig{
				Mapping: map[string]string{
					"alexmorbo": "alex.keep",
				},
			},
		}

		keepUser, ok := cfg.GetKeepUsername("unknown_user")
		assert.False(t, ok)
		assert.Equal(t, "", keepUser)
	})

	t.Run("returns false when mapping is nil", func(t *testing.T) {
		cfg := &FileConfig{}

		keepUser, ok := cfg.GetKeepUsername("alexmorbo")
		assert.False(t, ok)
		assert.Equal(t, "", keepUser)
	})

	t.Run("returns false when mapping is empty", func(t *testing.T) {
		cfg := &FileConfig{
			Users: UsersConfig{
				Mapping: map[string]string{},
			},
		}

		keepUser, ok := cfg.GetKeepUsername("alexmorbo")
		assert.False(t, ok)
		assert.Equal(t, "", keepUser)
	})

	t.Run("returns true with empty string when explicitly mapped to empty", func(t *testing.T) {
		cfg := &FileConfig{
			Users: UsersConfig{
				Mapping: map[string]string{
					"alexmorbo": "",
				},
			},
		}

		keepUser, ok := cfg.GetKeepUsername("alexmorbo")
		assert.True(t, ok)
		assert.Equal(t, "", keepUser)
	})

	t.Run("handles special characters in username", func(t *testing.T) {
		cfg := &FileConfig{
			Users: UsersConfig{
				Mapping: map[string]string{
					"user.name":   "keep.user",
					"user-name":   "keep-user",
					"user_name":   "keep_user",
					"user@domain": "keepuser",
				},
			},
		}

		keepUser, ok := cfg.GetKeepUsername("user.name")
		assert.True(t, ok)
		assert.Equal(t, "keep.user", keepUser)

		keepUser, ok = cfg.GetKeepUsername("user-name")
		assert.True(t, ok)
		assert.Equal(t, "keep-user", keepUser)

		keepUser, ok = cfg.GetKeepUsername("user_name")
		assert.True(t, ok)
		assert.Equal(t, "keep_user", keepUser)

		keepUser, ok = cfg.GetKeepUsername("user@domain")
		assert.True(t, ok)
		assert.Equal(t, "keepuser", keepUser)
	})
}

func TestServerConfigAddr(t *testing.T) {
	tests := []struct {
		name         string
		port         int
		expectedAddr string
	}{
		{
			name:         "default port",
			port:         8080,
			expectedAddr: "0.0.0.0:8080",
		},
		{
			name:         "custom port",
			port:         9090,
			expectedAddr: "0.0.0.0:9090",
		},
		{
			name:         "low port",
			port:         80,
			expectedAddr: "0.0.0.0:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServerConfig{Port: tt.port}
			assert.Equal(t, tt.expectedAddr, cfg.Addr())
		})
	}
}

func TestIsLabelGroupingEnabled(t *testing.T) {
	t.Run("returns true when enabled", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Grouping: LabelGroupingConfig{
					Enabled: true,
				},
			},
		}
		assert.True(t, cfg.IsLabelGroupingEnabled())
	})

	t.Run("returns false when disabled", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Grouping: LabelGroupingConfig{
					Enabled: false,
				},
			},
		}
		assert.False(t, cfg.IsLabelGroupingEnabled())
	})

	t.Run("returns false by default", func(t *testing.T) {
		cfg := &FileConfig{}
		assert.False(t, cfg.IsLabelGroupingEnabled())
	})
}

func TestGetLabelGroupingThreshold(t *testing.T) {
	t.Run("returns configured threshold", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Grouping: LabelGroupingConfig{
					Threshold: 5,
				},
			},
		}
		assert.Equal(t, 5, cfg.GetLabelGroupingThreshold())
	})

	t.Run("returns default 2 when threshold is 0 after applyDefaults", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Grouping: LabelGroupingConfig{
					Threshold: 0,
				},
			},
		}
		cfg.applyDefaults()
		assert.Equal(t, 2, cfg.GetLabelGroupingThreshold())
	})

	t.Run("returns default 2 for empty config after applyDefaults", func(t *testing.T) {
		cfg := &FileConfig{}
		cfg.applyDefaults()
		assert.Equal(t, 2, cfg.GetLabelGroupingThreshold())
	})
}

func TestGetLabelGroups(t *testing.T) {
	t.Run("returns configured groups", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Grouping: LabelGroupingConfig{
					Groups: []LabelGroupRule{
						{Prefixes: []string{"topology_"}, GroupName: "Topology", Priority: 100},
						{Prefixes: []string{"kubernetes_io_"}, GroupName: "Kubernetes", Priority: 90},
					},
				},
			},
		}

		groups := cfg.GetLabelGroups()
		require.Len(t, groups, 2)
		assert.Equal(t, "Topology", groups[0].GroupName)
		assert.Equal(t, 100, groups[0].Priority)
		assert.Equal(t, []string{"topology_"}, groups[0].Prefixes)
		assert.Equal(t, "Kubernetes", groups[1].GroupName)
	})

	t.Run("returns empty slice for no groups", func(t *testing.T) {
		cfg := &FileConfig{}
		groups := cfg.GetLabelGroups()
		assert.Empty(t, groups)
	})

	t.Run("returns groups with multiple prefixes", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Grouping: LabelGroupingConfig{
					Groups: []LabelGroupRule{
						{
							Prefixes:  []string{"kubernetes_io_", "beta_kubernetes_io_"},
							GroupName: "Kubernetes",
							Priority:  90,
						},
					},
				},
			},
		}

		groups := cfg.GetLabelGroups()
		require.Len(t, groups, 1)
		assert.Len(t, groups[0].Prefixes, 2)
	})
}

func TestLoadFromFileWithGroupingConfig(t *testing.T) {
	yamlContent := `
labels:
  grouping:
    enabled: true
    threshold: 3
    groups:
      - prefixes:
          - "topology_"
        group_name: "Topology"
        priority: 100
      - prefixes:
          - "kubernetes_io_"
          - "beta_kubernetes_io_"
        group_name: "Kubernetes"
        priority: 90
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	assert.True(t, cfg.IsLabelGroupingEnabled())
	assert.Equal(t, 3, cfg.GetLabelGroupingThreshold())

	groups := cfg.GetLabelGroups()
	require.Len(t, groups, 2)
	assert.Equal(t, "Topology", groups[0].GroupName)
	assert.Equal(t, "Kubernetes", groups[1].GroupName)
}

func TestShowSeverityField(t *testing.T) {
	tests := []struct {
		name     string
		config   *FileConfig
		expected bool
	}{
		{
			name:     "nil pointer returns true (default)",
			config:   &FileConfig{},
			expected: true,
		},
		{
			name: "explicit true returns true",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						ShowSeverity: boolPtr(true),
					},
				},
			},
			expected: true,
		},
		{
			name: "explicit false returns false",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						ShowSeverity: boolPtr(false),
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ShowSeverityField()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShowDescriptionField(t *testing.T) {
	tests := []struct {
		name     string
		config   *FileConfig
		expected bool
	}{
		{
			name:     "nil pointer returns true (default)",
			config:   &FileConfig{},
			expected: true,
		},
		{
			name: "explicit true returns true",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						ShowDescription: boolPtr(true),
					},
				},
			},
			expected: true,
		},
		{
			name: "explicit false returns false",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						ShowDescription: boolPtr(false),
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ShowDescriptionField()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSeverityFieldPosition(t *testing.T) {
	tests := []struct {
		name     string
		config   *FileConfig
		expected string
	}{
		{
			name:     "empty string returns first (default)",
			config:   &FileConfig{},
			expected: "first",
		},
		{
			name: "first returns first",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						SeverityPosition: "first",
					},
				},
			},
			expected: "first",
		},
		{
			name: "after_display returns after_display",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						SeverityPosition: "after_display",
					},
				},
			},
			expected: "after_display",
		},
		{
			name: "last returns last",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						SeverityPosition: "last",
					},
				},
			},
			expected: "last",
		},
		{
			name: "invalid value returns first (fallback)",
			config: &FileConfig{
				Message: MessageConfig{
					Fields: FieldsConfig{
						SeverityPosition: "invalid",
					},
				},
			},
			expected: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.SeverityFieldPosition()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultDisplayLabels(t *testing.T) {
	cfg := &FileConfig{}
	cfg.applyDefaults()

	expectedLabels := []string{
		"alertgroup",
		"container",
		"node",
		"namespace",
		"pod",
	}

	assert.Equal(t, expectedLabels, cfg.Labels.Display)
	assert.Len(t, cfg.Labels.Display, 5)
}

func TestDefaultExcludeLabels(t *testing.T) {
	cfg := &FileConfig{}
	cfg.applyDefaults()

	expectedLabels := []string{
		"__name__",
		"prometheus",
		"alertname",
		"job",
		"instance",
	}

	assert.Equal(t, expectedLabels, cfg.Labels.Exclude)
	assert.Len(t, cfg.Labels.Exclude, 5)
}

func TestDefaultRenameLabels(t *testing.T) {
	cfg := &FileConfig{}
	cfg.applyDefaults()

	assert.NotNil(t, cfg.Labels.Rename)
	assert.Equal(t, "Alert Group", cfg.Labels.Rename["alertgroup"])
	assert.Len(t, cfg.Labels.Rename, 1)
}

func TestValidate(t *testing.T) {
	t.Run("valid patterns pass validation", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{
					"exact_match",
					"prefix*",
					"*suffix",
					"*middle*",
					"node[0-9]",
					"label?",
				},
			},
		}

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty patterns pass validation", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{},
			},
		}

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil patterns pass validation", func(t *testing.T) {
		cfg := &FileConfig{}

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("unclosed bracket fails validation", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"[unclosed"},
			},
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid label exclude pattern")
		assert.Contains(t, err.Error(), "[unclosed")
	})

	t.Run("first invalid pattern is reported", func(t *testing.T) {
		cfg := &FileConfig{
			Labels: LabelsConfig{
				Exclude: []string{"valid*", "[invalid", "also[invalid"},
			},
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "[invalid")
	})
}

func TestLoadFromFileWithInvalidPattern(t *testing.T) {
	yamlContent := `
labels:
  exclude:
    - "[unclosed_bracket"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid_pattern.yaml")
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid label exclude pattern")
}

func boolPtr(b bool) *bool {
	return &b
}
