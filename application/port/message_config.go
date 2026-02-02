package port

type MessageConfig interface {
	ColorForSeverity(severity string) string
	EmojiForSeverity(severity string) string
	IsLabelExcluded(label string) bool
	IsLabelDisplayed(label string) bool
	RenameLabel(label string) string
	FooterText() string
	FooterIconURL() string
}
