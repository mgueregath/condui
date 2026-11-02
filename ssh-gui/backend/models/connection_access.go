package models

// NoVNCConfig is independent of direct VNC and uses an RFB WebSocket endpoint.
type NoVNCConfig struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

// VNCConfig stores both independent VNC access modes in the existing JSON config.
type VNCConfig struct {
	WebSocket *NoVNCConfig `json:"websocket,omitempty"`
	Enabled   bool         `json:"enabled"`
	Port      int          `json:"port"`
	Username  string       `json:"username"`
	Password  *string      `json:"password,omitempty"`
	Reuse     VNCReuse     `json:"reuse"`
}
type VNCReuse struct {
	User bool `json:"user"`
	Pass bool `json:"pass"`
}

func (c Connection) HasSSH() bool {
	if c.SSHEnabled != nil {
		return *c.SSHEnabled
	}
	return c.ConnectionProtocol() == "ssh"
}
func (c Connection) HasNoVNC() bool {
	return c.VNC != nil && c.VNC.WebSocket != nil && c.VNC.WebSocket.Enabled
}

func (c Connection) HasVNC() bool { return c.VNC != nil && c.VNC.Enabled }
