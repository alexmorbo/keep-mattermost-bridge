package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Server      ServerConfig
	Mattermost  MattermostConfig
	Keep        KeepConfig
	Redis       RedisConfig
	ConfigPath  string
	CallbackURL string
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
		ConfigPath:  getEnvOrDefault("CONFIG_PATH", "/etc/kmbridge/config.yaml"),
		CallbackURL: os.Getenv("CALLBACK_URL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
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
