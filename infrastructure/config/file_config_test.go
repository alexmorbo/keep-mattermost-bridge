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
