package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"ssh-gui/backend/account"
	"ssh-gui/backend/models"
	"ssh-gui/backend/storage"
)


// ============================================================
// Vault methods
// ============================================================

// IsVaultSetup returns true if a master password salt has been stored.
func (a *App) IsVaultSetup() bool {
	salt, _ := a.database.GetSetting("master_key_salt")
	return salt != ""
}

// IsVaultLocked returns true if the master key is not loaded in memory.
func (a *App) IsVaultLocked() bool {
	return a.getMasterKey() == nil
}

// SetupMasterPassword creates the vault for the first time.
func (a *App) SetupMasterPassword(password string) error {
	if a.IsVaultSetup() {
		return fmt.Errorf("vault already set up")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	salt, err := account.GenerateSalt()
	if err != nil {
		return err
	}

	key := account.DeriveKey(password, salt)

	testCipher, err := account.MakeVaultTest(key)
	if err != nil {
		return err
	}

	saltB64 := base64.StdEncoding.EncodeToString(salt)
	if err := a.database.SetSetting("master_key_salt", saltB64); err != nil {
		return err
	}
	if err := a.database.SetSetting("vault_test", testCipher); err != nil {
		return err
	}

	a.setMasterKey(key)
	return nil
}

// UnlockVault derives the master key from password and verifies it.
func (a *App) UnlockVault(password string) error {
	saltB64, _ := a.database.GetSetting("master_key_salt")
	if saltB64 == "" {
		return fmt.Errorf("vault not set up")
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return fmt.Errorf("invalid vault salt")
	}

	key := account.DeriveKey(password, salt)

	testCipher, _ := a.database.GetSetting("vault_test")
	if testCipher == "" {
		return fmt.Errorf("vault verification data missing")
	}
	if !account.VerifyVaultTest(key, testCipher) {
		return fmt.Errorf("incorrect master password")
	}

	a.setMasterKey(key)
	return nil
}

// LockVault clears the master key from memory.
func (a *App) LockVault() {
	a.clearMasterKey()
}

// ============================================================
// Account methods
// ============================================================

// GetAccountStatus returns the current account and sync status.
func (a *App) GetAccountStatus() account.AccountStatus {
	lastSync, _ := a.database.GetSetting("last_sync_at")
	return a.accountManager.GetStatus(lastSync)
}

// AccountRegister creates a new account on the sync server.
func (a *App) AccountRegister(serverURL, email, password string) error {
	return a.accountManager.Register(serverURL, email, password)
}

// AccountLogin authenticates against the sync server.
func (a *App) AccountLogin(serverURL, email, password string) error {
	state, err := a.accountManager.Login(serverURL, email, password)
	if err != nil {
		return err
	}
	if err := a.saveAccountState(state); err != nil {
		return err
	}
	// Immediately refresh to confirm tier from server and start the loop
	go a.doTokenRefresh()
	return nil
}

// AccountLogout logs out and clears local account data.
func (a *App) AccountLogout() error {
	// Stop the refresh loop so it doesn't try to refresh with a stale token
	if a.cancelTokenRefresh != nil {
		a.cancelTokenRefresh()
	}
	serverURL, token, rt := a.accountManager.Logout()
	// Best-effort server-side logout in background
	if token != "" {
		go a.accountManager.DoServerLogout(serverURL, token, rt)
	}
	return a.database.ClearAccount()
}

// SyncNow encrypts all local connections and uploads to the sync server.
func (a *App) SyncNow() error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	if !a.accountManager.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync account")
	}

	if a.accountManager.GetPublicKey() == "" {
		state, err := a.accountManager.SetupIdentity(key)
		if err != nil {
			return fmt.Errorf("failed to set up identity: %w", err)
		}
		if err := a.saveAccountState(state); err != nil {
			return err
		}
	}

	connections, err := a.database.GetConnections()
	if err != nil {
		return err
	}

	// Free tier: cap remote sync at 5 connections (local storage is unlimited)
	const freeSyncLimit = 5
	if a.accountManager.GetState().Tier == "free" && len(connections) > freeSyncLimit {
		connections = connections[:freeSyncLimit]
	}

	data, err := json.Marshal(connections)
	if err != nil {
		return err
	}

	if err := a.accountManager.SyncConnections(data, key); err != nil {
		return err
	}

	return a.database.SetSetting("last_sync_at", time.Now().Format(time.RFC3339))
}

// GetPublicKey returns this account's X25519 public key.
func (a *App) GetPublicKey() string {
	return a.accountManager.GetPublicKey()
}

// ShareConnection shares a connection with another Condui user by email.
func (a *App) ShareConnection(connectionID, recipientEmail string, readOnly bool) error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	if !a.accountManager.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync account")
	}

	conn, err := a.database.GetConnectionByID(connectionID)
	if err != nil {
		return err
	}

	// Sanitize: strip password from shared copy (recipient enters their own creds)
	shareable := *conn
	shareable.Password = nil

	data, err := json.Marshal(shareable)
	if err != nil {
		return err
	}

	return a.accountManager.ShareConnection(data, recipientEmail, readOnly, key)
}

// GetIncomingShares returns pending/accepted share invites.
func (a *App) GetIncomingShares() ([]account.ShareInfo, error) {
	return a.accountManager.GetIncomingShares()
}

// AcceptShare accepts an incoming share invite and imports the connection locally.
func (a *App) AcceptShare(shareID, encryptedKey, blobID string) error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}

	connectionJSON, err := a.accountManager.AcceptShare(shareID, encryptedKey, blobID, key)
	if err != nil {
		return err
	}

	var conn models.Connection
	if err := json.Unmarshal(connectionJSON, &conn); err != nil {
		return fmt.Errorf("invalid shared connection data: %w", err)
	}

	conn.ID = "" // will be assigned by CreateConnection
	conn.Password = nil // recipient starts without a password for this connection
	sharedName := "[Shared] " + conn.Name
	conn.Name = sharedName

	return a.database.CreateConnection(&conn)
}

// ============================================================
// Helpers
// ============================================================

func (a *App) saveAccountState(s account.State) error {
	return a.database.SaveAccount(&storage.AccountRow{
		ServerURL:    s.ServerURL,
		UserID:       s.UserID,
		Email:        s.Email,
		Tier:         s.Tier,
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		PublicKey:    s.PublicKey,
		IdentityBlob: s.IdentityBlob,
	})
}
