package main

import (
	"encoding/hex"
	"fmt"
	"strings"

	"ssh-gui/backend/models"
)

func (a *App) validateConnection(c *models.Connection) error {
	if err := c.ValidateProtocol(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("name and host are required")
	}
	if !c.HasSSH() && !c.HasVNC() && !c.HasRDP() {
		return fmt.Errorf("enable at least one connection access")
	}
	if c.HasSSH() {
		c.Protocol = "ssh"
		if c.Port < 1 || c.Port > 65535 || strings.TrimSpace(c.Username) == "" {
			return fmt.Errorf("SSH username and a valid port are required")
		}
		if c.AuthType != "password" && c.AuthType != "private_key" {
			return fmt.Errorf("unsupported SSH authentication")
		}
		if c.AuthType == "private_key" && strings.TrimSpace(strVal(c.PrivateKeyPath)) == "" {
			return fmt.Errorf("SSH private key path is required")
		}
		if strVal(c.JumpHostID) != "" {
			if *c.JumpHostID == c.ID && c.ID != "" {
				return fmt.Errorf("connection cannot use itself as jump host")
			}
			jump, err := a.database.GetConnectionByID(*c.JumpHostID)
			if err != nil {
				return err
			}
			if !jump.HasSSH() {
				return fmt.Errorf("jump host must enable SSH")
			}
		}
	} else {
		if c.HasVNC() {
			c.Protocol = "vnc"
		} else {
			c.Protocol = "rdp"
		}
	}
	if c.HasVNC() {
		if c.VNC.Port < 1 || c.VNC.Port > 65535 {
			return fmt.Errorf("invalid VNC port")
		}
		if (c.VNC.Reuse.User || c.VNC.Reuse.Pass) && !c.HasSSH() {
			return fmt.Errorf("VNC cannot reuse disabled SSH credentials")
		}
		if c.VNC.Reuse.Pass && c.AuthType != "password" {
			return fmt.Errorf("VNC cannot reuse an SSH private key or passphrase")
		}
	}
	if c.HasRDP() {
		if _, _, err := c.RDP.DesktopSize(); err != nil {
			return err
		}
		fingerprint := strings.ReplaceAll(strings.TrimSpace(c.RDP.CertificateFingerprint), ":", "")
		if !c.RDP.DisableTLS && !c.RDP.AllowUntrusted && fingerprint != "" {
			decoded, err := hex.DecodeString(fingerprint)
			if err != nil || len(decoded) != 32 {
				return fmt.Errorf("invalid RDP certificate fingerprint")
			}
		}

		if c.RDP.Port < 1 || c.RDP.Port > 65535 {
			return fmt.Errorf("invalid RDP port")
		}
		if (c.RDP.Reuse.User || c.RDP.Reuse.Pass) && !c.HasSSH() {
			return fmt.Errorf("RDP cannot reuse disabled SSH credentials")
		}
		if c.RDP.Reuse.Pass && c.AuthType != "password" {
			return fmt.Errorf("RDP cannot reuse private keys or passphrases")
		}
		user := c.RDP.Username
		if c.RDP.Reuse.User {
			user = c.Username
		}
		if strings.TrimSpace(user) == "" {
			return fmt.Errorf("RDP username is required")
		}
	}
	return nil
}
