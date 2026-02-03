package post

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
