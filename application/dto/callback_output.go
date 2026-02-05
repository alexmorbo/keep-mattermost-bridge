package dto

import "github.com/alexmorbo/keep-mattermost-bridge/domain/post"

type CallbackOutput struct {
	Attachment AttachmentDTO
	Ephemeral  string
}

type AttachmentDTO struct {
	Color      string
	Title      string
	TitleLink  string
	Text       string
	Fields     []AttachmentFieldDTO
	Actions    []ButtonDTO
	Footer     string
	FooterIcon string
}

type AttachmentFieldDTO struct {
	Title string
	Value string
	Short bool
}

type ButtonDTO struct {
	ID          string
	Name        string
	Style       string
	Integration ButtonIntegrationDTO
}

type ButtonIntegrationDTO struct {
	URL     string
	Context map[string]string
}

func NewAttachmentDTO(a post.Attachment) AttachmentDTO {
	fields := make([]AttachmentFieldDTO, len(a.Fields))
	for i, f := range a.Fields {
		fields[i] = AttachmentFieldDTO{Title: f.Title, Value: f.Value, Short: f.Short}
	}

	buttons := make([]ButtonDTO, len(a.Actions))
	for i, b := range a.Actions {
		buttons[i] = ButtonDTO{
			ID:    b.ID,
			Name:  b.Name,
			Style: b.Style,
			Integration: ButtonIntegrationDTO{
				URL:     b.Integration.URL,
				Context: b.Integration.Context,
			},
		}
	}

	return AttachmentDTO{
		Color:      a.Color,
		Title:      a.Title,
		TitleLink:  a.TitleLink,
		Text:       a.Text,
		Fields:     fields,
		Actions:    buttons,
		Footer:     a.Footer,
		FooterIcon: a.FooterIcon,
	}
}
