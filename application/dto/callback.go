package dto

type MattermostCallbackInput struct {
	UserID    string            `json:"user_id"`
	PostID    string            `json:"post_id"`
	ChannelID string            `json:"channel_id"`
	Context   map[string]string `json:"context"`
}
