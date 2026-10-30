package main

import (
	"fmt"

	"ssh-gui/backend/models"
)

func redactConnection(c *models.Connection) {
	if c.Password != nil && isSyncEncrypted(*c.Password) {
		c.PasswordPending = true
	}
	c.Password = nil
	c.Passphrase = nil
	if c.VNC != nil {
		c.VNC.Password = nil
	}
}
func (a *App) encryptVNC(c *models.Connection, stored *models.Connection) error {
	if c.VNC == nil {
		if stored != nil {
			c.VNC = stored.VNC
		}
		return nil
	}
	if c.VNC.Password == nil {
		if stored != nil && stored.VNC != nil {
			c.VNC.Password = stored.VNC.Password
		}
		return nil
	}
	if *c.VNC.Password == "" {
		return nil
	}
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	encrypted, err := encryptConnectionPassword(*c.VNC.Password, key)
	if err != nil {
		return err
	}
	c.VNC.Password = &encrypted
	return nil
}
func (a *App) resolveVNC(c models.Connection, stored *models.Connection) (string, string, error) {
	if !c.HasVNC() || c.ShareRevoked {
		return "", "", fmt.Errorf("VNC access is unavailable")
	}
	if c.Host == "" || c.VNC.Port < 1 || c.VNC.Port > 65535 {
		return "", "", fmt.Errorf("invalid VNC host or port")
	}
	if (c.VNC.Reuse.User || c.VNC.Reuse.Pass) && !c.HasSSH() {
		return "", "", fmt.Errorf("SSH access is disabled")
	}
	if c.VNC.Reuse.Pass && c.AuthType != "password" {
		return "", "", fmt.Errorf("VNC cannot reuse an SSH private key or passphrase")
	}
	key := a.getMasterKey()
	if key == nil {
		return "", "", fmt.Errorf("vault is locked")
	}
	username := c.VNC.Username
	if c.VNC.Reuse.User {
		username = c.Username
	}
	secret := c.VNC.Password
	saved := false
	if c.VNC.Reuse.Pass {
		secret = c.Password
		if secret == nil && stored != nil {
			secret = stored.Password
			saved = true
		}
	} else if secret == nil && stored != nil && stored.VNC != nil {
		secret = stored.VNC.Password
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
