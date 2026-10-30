package models

// VNCConfig belongs to the same saved connection and shares its Host.
type VNCConfig struct {
	Enabled  bool     `json:"enabled"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password *string  `json:"password,omitempty"`
	Reuse    VNCReuse `json:"reuse"`
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
func (c Connection) HasVNC() bool { return c.VNC != nil && c.VNC.Enabled }
