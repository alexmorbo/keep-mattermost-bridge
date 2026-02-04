package port

type UserMapper interface {
	GetKeepUsername(mattermostUsername string) string
}
