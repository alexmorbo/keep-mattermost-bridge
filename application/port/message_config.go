package port

type LabelGroupConfig struct {
	Prefixes  []string
	GroupName string
	Priority  int
}

type MessageConfig interface {
	ColorForSeverity(severity string) string
	EmojiForSeverity(severity string) string
	IsLabelExcluded(label string) bool
	IsLabelDisplayed(label string) bool
	RenameLabel(label string) string
	FooterText() string
	FooterIconURL() string
	IsLabelGroupingEnabled() bool
	GetLabelGroupingThreshold() int
	GetLabelGroups() []LabelGroupConfig
	ShowSeverityField() bool
	ShowDescriptionField() bool
	SeverityFieldPosition() string
}
