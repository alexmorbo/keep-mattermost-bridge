package post

import "encoding/json"

type Attachment struct {
	Color      string
	Title      string
	TitleLink  string
	Text       string
	Fields     []AttachmentField
	Actions    []Button
	Footer     string
	FooterIcon string
}

type AttachmentField struct {
	Title string
	Value string
	Short bool
}

func (a *Attachment) ToJSON() (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func AttachmentFromJSON(data string) (*Attachment, error) {
	var attachment Attachment
	if err := json.Unmarshal([]byte(data), &attachment); err != nil {
		return nil, err
	}
	return &attachment, nil
}
