package port

type ChannelResolver interface {
	ChannelIDForSeverity(severity string) string
}
