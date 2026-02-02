package dto

type MattermostCallbackInput struct {
	UserID  string            `json:"user_id"`
	Context map[string]string `json:"context"`
}
