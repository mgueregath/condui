package models

import "fmt"

// RDPConfig uses the shared connection Host; credentials remain local.
type RDPConfig struct {
	AllowUntrusted         bool     `json:"allowUntrusted,omitempty"`
	DisableTLS             bool     `json:"disableTLS,omitempty"`
	Enabled                bool     `json:"enabled"`
	Resolution             string   `json:"resolution,omitempty"`
	Port                   int      `json:"port"`
	Username               string   `json:"username"`
	Domain                 string   `json:"domain"`
	Password               *string  `json:"password,omitempty"`
	Reuse                  VNCReuse `json:"reuse"`
	CertificateFingerprint string   `json:"certificateFingerprint,omitempty"`
}

func (c Connection) HasRDP() bool { return c.RDP != nil && c.RDP.Enabled }

// DesktopSize resolves supported presets and defaults existing configurations.
func (c RDPConfig) DesktopSize() (uint16, uint16, error) {
	switch c.Resolution {
	case "1280x720":
		return 1280, 720, nil
	case "", "1920x1080":
		return 1920, 1080, nil
	case "2560x1440":
		return 2560, 1440, nil
	default:
		return 0, 0, fmt.Errorf("unsupported RDP resolution")
	}
}
