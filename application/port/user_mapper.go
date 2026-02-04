package port

// UserMapper translates Mattermost usernames to Keep usernames.
// Used to assign alerts to the corresponding Keep user when
// a Mattermost user acknowledges or resolves an alert.
type UserMapper interface {
	// GetKeepUsername returns the Keep username for a given Mattermost username.
	// Returns the Keep username and true if mapping exists, or empty string and false if not found.
	GetKeepUsername(mattermostUsername string) (string, bool)
}
