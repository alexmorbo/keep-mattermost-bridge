package port

// UserMapper translates between Mattermost and Keep usernames.
// Used to assign alerts to the corresponding Keep user when
// a Mattermost user acknowledges or resolves an alert.
type UserMapper interface {
	// GetKeepUsername returns the Keep username for a given Mattermost username.
	// Returns the Keep username and true if mapping exists, or empty string and false if not found.
	GetKeepUsername(mattermostUsername string) (string, bool)

	// GetMattermostUsername returns the Mattermost username for a given Keep username.
	// Used to display the correct Mattermost username when processing webhooks from Keep.
	// Returns the Mattermost username and true if mapping exists, or empty string and false if not found.
	GetMattermostUsername(keepUsername string) (string, bool)
}
