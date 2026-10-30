package main

import (
	"fmt"

	"ssh-gui/backend/models"
)

func (a *App) encryptRDP(c *models.Connection, stored *models.Connection) error {
	if c.RDP == nil {
		if stored != nil {
			c.RDP = stored.RDP
		}
		return nil
	}
	if c.RDP.Password == nil {
		if stored != nil && stored.RDP != nil {
			c.RDP.Password = stored.RDP.Password
		}
		return nil
	}
	if *c.RDP.Password == "" {
		return nil
	}
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	encrypted, err := encryptConnectionPassword(*c.RDP.Password, key)
	if err != nil {
		return err
	}
	c.RDP.Password = &encrypted
	return nil
}
func (a *App) resolveRDP(c models.Connection, stored *models.Connection) (string, string, error) {
	if !c.HasRDP() || c.ShareRevoked {
		return "", "", fmt.Errorf("RDP access is unavailable")
	}
	if c.Host == "" || c.RDP.Port < 1 || c.RDP.Port > 65535 {
		return "", "", fmt.Errorf("invalid RDP host or port")
	}
	if (c.RDP.Reuse.User || c.RDP.Reuse.Pass) && !c.HasSSH() {
		return "", "", fmt.Errorf("SSH access is disabled")
	}
	if c.RDP.Reuse.Pass && c.AuthType != "password" {
		return "", "", fmt.Errorf("RDP cannot reuse an SSH private key or passphrase")
	}
	key := a.getMasterKey()
	if key == nil {
		return "", "", fmt.Errorf("vault is locked")
	}
	username := c.RDP.Username
	if c.RDP.Reuse.User {
		username = c.Username
	}
	secret := c.RDP.Password
	saved := false
	if c.RDP.Reuse.Pass {
		secret = c.Password
		if secret == nil && stored != nil {
			secret = stored.Password
			saved = true
		}
	} else if secret == nil && stored != nil && stored.RDP != nil {
		secret = stored.RDP.Password
		saved = true
	}
	if secret == nil {
		return username, "", nil
	}
	if isSyncEncrypted(*secret) {
		return "", "", fmt.Errorf("connection password is pending vault decryption")
	}
	if saved || isConnectionEncrypted(*secret) {
		password, err := decryptConnectionPassword(*secret, key)
		if err != nil {
			return "", "", fmt.Errorf("unable to decrypt connection password")
		}
		return username, password, nil
	}
	return username, *secret, nil
}
