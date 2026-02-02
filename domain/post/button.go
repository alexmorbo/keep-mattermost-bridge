package post

type Button struct {
	ID          string
	Name        string
	Integration ButtonIntegration
}

type ButtonIntegration struct {
	URL     string
	Context map[string]string
}
