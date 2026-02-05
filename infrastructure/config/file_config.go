package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/alexmorbo/keep-mattermost-bridge/application/port"
	"github.com/alexmorbo/keep-mattermost-bridge/domain/post"
)

type FileConfig struct {
	Channels ChannelsConfig `yaml:"channels"`
	Message  MessageConfig  `yaml:"message"`
	Labels   LabelsConfig   `yaml:"labels"`
	Users    UsersConfig    `yaml:"users"`
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
	Fields FieldsConfig      `yaml:"fields"`
}

type FieldsConfig struct {
	ShowSeverity     *bool  `yaml:"show_severity"`
	ShowDescription  *bool  `yaml:"show_description"`
	SeverityPosition string `yaml:"severity_position"`
}

type FooterConfig struct {
	Text    string `yaml:"text"`
	IconURL string `yaml:"icon_url"`
}

type LabelsConfig struct {
	Display  []string            `yaml:"display"`
	Rename   map[string]string   `yaml:"rename"`
	Exclude  []string            `yaml:"exclude"`
	Grouping LabelGroupingConfig `yaml:"grouping"`
}

type LabelGroupingConfig struct {
	Enabled   bool             `yaml:"enabled"`
	Threshold int              `yaml:"threshold"` // default: 2
	Groups    []LabelGroupRule `yaml:"groups"`
}

type LabelGroupRule struct {
	Prefixes  []string `yaml:"prefixes"`
	GroupName string   `yaml:"group_name"`
	Priority  int      `yaml:"priority"`
}

type UsersConfig struct {
	Mapping map[string]string `yaml:"mapping"`
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

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func defaultFileConfig() *FileConfig {
	cfg := &FileConfig{}
	cfg.applyDefaults()
	return cfg
}

func (c *FileConfig) Validate() error {
	for _, pattern := range c.Labels.Exclude {
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("invalid label exclude pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func (c *FileConfig) applyDefaults() {
	if c.Message.Colors == nil {
		c.Message.Colors = map[string]string{
			"critical":     "#CC0000",
			"high":         "#FF6600",
			"warning":      "#EDA200",
			"info":         "#0066FF",
			"low":          "#808080",
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
			"low":      "⚪",
		}
	}
	if c.Message.Footer.Text == "" {
		c.Message.Footer.Text = "Keep AIOps"
	}
	if c.Message.Footer.IconURL == "" {
		c.Message.Footer.IconURL = "https://avatars.githubusercontent.com/u/109032290?v=4"
	}
	if c.Message.Fields.SeverityPosition == "" {
		c.Message.Fields.SeverityPosition = post.SeverityPositionFirst
	}
	if c.Labels.Display == nil {
		c.Labels.Display = []string{
			"alertgroup",
			"container",
			"node",
			"namespace",
			"pod",
		}
	}
	if c.Labels.Exclude == nil {
		c.Labels.Exclude = []string{
			"__name__",
			"prometheus",
			"alertname",
			"job",
			"instance",
		}
	}
	if c.Labels.Rename == nil {
		c.Labels.Rename = map[string]string{
			"alertgroup": "Alert Group",
		}
	}
	// Default grouping config
	if c.Labels.Grouping.Threshold == 0 {
		c.Labels.Grouping.Threshold = 2
	}
	if c.Labels.Grouping.Groups == nil && c.Labels.Grouping.Enabled {
		c.Labels.Grouping.Groups = []LabelGroupRule{
			{
				Prefixes:  []string{"topology_"},
				GroupName: "Topology",
				Priority:  100,
			},
			{
				Prefixes:  []string{"kubernetes_io_", "beta_kubernetes_io_", "failure_domain_beta_kubernetes_io_"},
				GroupName: "Kubernetes",
				Priority:  90,
			},
			{
				Prefixes:  []string{"extensions_talos_dev_"},
				GroupName: "Talos",
				Priority:  80,
			},
		}
	}
	if c.Users.Mapping == nil {
		c.Users.Mapping = make(map[string]string)
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
	for _, pattern := range c.Labels.Exclude {
		matched, err := path.Match(pattern, label)
		if err != nil {
			if pattern == label {
				return true
			}
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func (c *FileConfig) IsLabelDisplayed(label string) bool {
	if len(c.Labels.Display) == 0 {
		return !c.Labels.Grouping.Enabled
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

func (c *FileConfig) GetKeepUsername(mattermostUsername string) (string, bool) {
	if c.Users.Mapping == nil {
		return "", false
	}
	keepUser, ok := c.Users.Mapping[mattermostUsername]
	return keepUser, ok
}

func (c *FileConfig) IsLabelGroupingEnabled() bool {
	return c.Labels.Grouping.Enabled
}

func (c *FileConfig) GetLabelGroupingThreshold() int {
	return c.Labels.Grouping.Threshold
}

func (c *FileConfig) GetLabelGroups() []port.LabelGroupConfig {
	result := make([]port.LabelGroupConfig, len(c.Labels.Grouping.Groups))
	for i, g := range c.Labels.Grouping.Groups {
		result[i] = port.LabelGroupConfig{
			Prefixes:  g.Prefixes,
			GroupName: g.GroupName,
			Priority:  g.Priority,
		}
	}
	return result
}

func (c *FileConfig) ShowSeverityField() bool {
	if c.Message.Fields.ShowSeverity == nil {
		return true
	}
	return *c.Message.Fields.ShowSeverity
}

func (c *FileConfig) ShowDescriptionField() bool {
	if c.Message.Fields.ShowDescription == nil {
		return true
	}
	return *c.Message.Fields.ShowDescription
}

func (c *FileConfig) SeverityFieldPosition() string {
	pos := c.Message.Fields.SeverityPosition
	if pos == "" {
		return post.SeverityPositionFirst
	}
	switch pos {
	case post.SeverityPositionFirst, post.SeverityPositionAfterDisplay, post.SeverityPositionLast:
		return pos
	default:
		return post.SeverityPositionFirst
	}
}
