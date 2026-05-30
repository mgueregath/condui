package sessions

type ConnectionRequest struct {
	ID       string
	Host     string
	Port     int
	Username string
	Password string
}
