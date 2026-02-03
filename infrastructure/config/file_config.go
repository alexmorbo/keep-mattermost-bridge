package config

import (
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

type FileConfig struct {
	Channels ChannelsConfig `yaml:"channels"`
	Message  MessageConfig  `yaml:"message"`
	Labels   LabelsConfig   `yaml:"labels"`
}

type ChannelsConfig struct {
	Routing          []RoutingRule `yaml:"routing"`
	DefaultChannelID string        `yaml:"default_channel_id"`
}

type RoutingRule struct {
	Severity  string `yaml:"severity"`
	ChannelID string `yaml:"channel_id"`
}

type MessageConfig struct {
	Colors map[string]string `yaml:"colors"`
	Emoji  map[string]string `yaml:"emoji"`
	Footer FooterConfig      `yaml:"footer"`
}

type FooterConfig struct {
	Text    string `yaml:"text"`
	IconURL string `yaml:"icon_url"`
}

type LabelsConfig struct {
	Display []string          `yaml:"display"`
	Rename  map[string]string `yaml:"rename"`
	Exclude []string          `yaml:"exclude"`
}

func LoadFromFile(path string) (*FileConfig, error) {
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultFileConfig(), nil
		}
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func defaultFileConfig() *FileConfig {
	cfg := &FileConfig{}
	cfg.applyDefaults()
	return cfg
}

func (c *FileConfig) applyDefaults() {
	if c.Message.Colors == nil {
		c.Message.Colors = map[string]string{
			"critical":     "#CC0000",
			"high":         "#FF6600",
			"warning":      "#EDA200",
			"info":         "#0066FF",
			"acknowledged": "#FFA500",
			"resolved":     "#00CC00",
		}
	}
	if c.Message.Emoji == nil {
		c.Message.Emoji = map[string]string{
			"critical": "🔴",
			"high":     "🟠",
			"warning":  "🟡",
			"info":     "🔵",
		}
	}
	if c.Message.Footer.Text == "" {
		c.Message.Footer.Text = "Keep AIOps"
	}
	if c.Message.Footer.IconURL == "" {
		c.Message.Footer.IconURL = "https://avatars.githubusercontent.com/u/109032290?v=4"
	}
	if c.Labels.Rename == nil {
		c.Labels.Rename = make(map[string]string)
	}
}

func (c *FileConfig) ChannelIDForSeverity(severity string) string {
	for _, rule := range c.Channels.Routing {
		if rule.Severity == severity {
			return rule.ChannelID
		}
	}
	return c.Channels.DefaultChannelID
}

func (c *FileConfig) ColorForSeverity(severity string) string {
	if color, ok := c.Message.Colors[severity]; ok {
		return color
	}
	return "#808080"
}

func (c *FileConfig) EmojiForSeverity(severity string) string {
	if emoji, ok := c.Message.Emoji[severity]; ok {
		return emoji
	}
	return ""
}

func (c *FileConfig) IsLabelExcluded(label string) bool {
	return slices.Contains(c.Labels.Exclude, label)
}

func (c *FileConfig) IsLabelDisplayed(label string) bool {
	if len(c.Labels.Display) == 0 {
		return true
	}
	return slices.Contains(c.Labels.Display, label)
}

func (c *FileConfig) RenameLabel(label string) string {
	if renamed, ok := c.Labels.Rename[label]; ok {
		return renamed
	}
	return label
}

func (c *FileConfig) FooterText() string {
	return c.Message.Footer.Text
}

func (c *FileConfig) FooterIconURL() string {
	return c.Message.Footer.IconURL
}
