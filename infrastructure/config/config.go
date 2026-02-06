package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server      ServerConfig
	Mattermost  MattermostConfig
	Keep        KeepConfig
	Redis       RedisConfig
	Polling     PollingConfig
	Setup       SetupConfig
	ConfigPath  string
	CallbackURL string
}

// SetupConfig configures automatic Keep provider and workflow creation.
type SetupConfig struct {
	Enabled bool // Create webhook provider and workflow on startup (default: true)
}

// PollingConfig configures background polling for detecting assignee changes
// made directly in Keep UI, which bypass webhook notifications.
type PollingConfig struct {
	Enabled     bool          // Enable background polling
	Interval    time.Duration // Interval between polling cycles (minimum 10s)
	AlertsLimit int           // Maximum alerts to fetch from Keep API per poll (default 1000)
}

type ServerConfig struct {
	Port     int
	LogLevel string
}

func (c *ServerConfig) Addr() string {
	return "0.0.0.0:" + strconv.Itoa(c.Port)
}

type MattermostConfig struct {
	URL   string
	Token string
}

type KeepConfig struct {
	URL    string
	APIKey string
	UIURL  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func LoadFromEnv() (*Config, error) {
	serverPort, err := getEnvOrDefaultInt("SERVER_PORT", 8080)
	if err != nil {
		return nil, err
	}

	redisDB, err := getEnvOrDefaultInt("REDIS_DB", 0)
	if err != nil {
		return nil, err
	}

	pollingEnabled, err := getEnvOrDefaultBool("POLLING_ENABLED", false)
	if err != nil {
		return nil, err
	}

	pollingInterval, err := getEnvOrDefaultDuration("POLLING_INTERVAL", time.Minute)
	if err != nil {
		return nil, err
	}

	pollingAlertsLimit, err := getEnvOrDefaultInt("POLLING_ALERTS_LIMIT", 1000)
	if err != nil {
		return nil, err
	}

	setupEnabled, err := getEnvOrDefaultBool("KEEP_SETUP_ENABLED", true)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:     serverPort,
			LogLevel: getEnvOrDefault("LOG_LEVEL", "info"),
		},
		Mattermost: MattermostConfig{
			URL:   os.Getenv("MATTERMOST_URL"),
			Token: os.Getenv("MATTERMOST_TOKEN"),
		},
		Keep: KeepConfig{
			URL:    os.Getenv("KEEP_URL"),
			APIKey: os.Getenv("KEEP_API_KEY"),
			UIURL:  os.Getenv("KEEP_UI_URL"),
		},
		Redis: RedisConfig{
			Addr:     getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		Polling: PollingConfig{
			Enabled:     pollingEnabled,
			Interval:    pollingInterval,
			AlertsLimit: pollingAlertsLimit,
		},
		Setup: SetupConfig{
			Enabled: setupEnabled,
		},
		ConfigPath:  getEnvOrDefault("CONFIG_PATH", "/etc/kmbridge/config.yaml"),
		CallbackURL: os.Getenv("CALLBACK_URL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ApplyFileConfig applies settings from FileConfig if env variables are not set.
// Env variables have priority over file config.
func (c *Config) ApplyFileConfig(fc *FileConfig) {
	// Polling: use file config if env not set
	if os.Getenv("POLLING_ENABLED") == "" && fc.Polling.Enabled != nil {
		c.Polling.Enabled = *fc.Polling.Enabled
	}
	if os.Getenv("POLLING_INTERVAL") == "" && fc.Polling.Interval != "" {
		if d, err := time.ParseDuration(fc.Polling.Interval); err == nil {
			c.Polling.Interval = d
		}
	}
	if os.Getenv("POLLING_ALERTS_LIMIT") == "" && fc.Polling.AlertsLimit != nil {
		c.Polling.AlertsLimit = *fc.Polling.AlertsLimit
	}

	// Setup: use file config if env not set
	if os.Getenv("KEEP_SETUP_ENABLED") == "" && fc.Setup.Enabled != nil {
		c.Setup.Enabled = *fc.Setup.Enabled
	}
}

func (c *Config) validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Mattermost.URL == "" {
		return fmt.Errorf("MATTERMOST_URL is required")
	}
	if c.Mattermost.Token == "" {
		return fmt.Errorf("MATTERMOST_TOKEN is required")
	}
	if c.Keep.URL == "" {
		return fmt.Errorf("KEEP_URL is required")
	}
	if c.Keep.APIKey == "" {
		return fmt.Errorf("KEEP_API_KEY is required")
	}
	if c.Keep.UIURL == "" {
		return fmt.Errorf("KEEP_UI_URL is required")
	}
	if c.CallbackURL == "" {
		return fmt.Errorf("CALLBACK_URL is required")
	}
	if c.Polling.Enabled {
		if c.Polling.Interval < 10*time.Second {
			return fmt.Errorf("POLLING_INTERVAL must be at least 10s when polling is enabled, got %s", c.Polling.Interval)
		}
		if c.Polling.AlertsLimit < 1 {
			return fmt.Errorf("POLLING_ALERTS_LIMIT must be at least 1, got %d", c.Polling.AlertsLimit)
		}
	}
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("parse %s=%q: %w", key, v, err)
	}
	return i, nil
}

func getEnvOrDefaultBool(key string, defaultValue bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("parse %s=%q: %w", key, v, err)
	}
	return b, nil
}

func getEnvOrDefaultDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("parse %s=%q: %w", key, v, err)
	}
	return d, nil
}
