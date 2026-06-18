package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"ssh-gui/backend/account"
	"ssh-gui/backend/models"
	"ssh-gui/backend/storage"
)

type connectionsSyncData struct {
	Connections []models.Connection `json:"connections"`
	Folders     []models.Folder     `json:"folders"`
}

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
	a.triggerBackgroundSync()
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
	a.triggerBackgroundSync()
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
	if a.getMasterKey() != nil {
		a.triggerBackgroundSync()
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

// SyncNow reconciles local and remote connections, then uploads the merged set.
func (a *App) SyncNow() error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	if !a.accountManager.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync account")
	}
	if err := a.ensureAccessTokenFresh(); err != nil {
		return fmt.Errorf("failed to refresh access token: %w", err)
	}

	state, err := a.accountManager.SetupIdentity(key)
	if err != nil {
		return fmt.Errorf("failed to set up identity: %w", err)
	}
	if err := a.saveAccountState(state); err != nil {
		return err
	}

	connections, err := a.database.GetConnections()
	if err != nil {
		return err
	}
	folders, err := a.database.GetFolders()
	if err != nil {
		return err
	}

	// Pro tier syncs the same connection set across all devices. Free tier
	// keeps the previous capped upload behavior.
	const freeSyncLimit = 5
	if a.accountManager.GetState().Tier == "pro" {
		shouldUpload := true
		localHash := syncSetHash(connections, folders)
		remoteMeta, metaErr := a.accountManager.GetConnectionsMeta()
		if metaErr != nil && !isMissingRemoteBlob(metaErr) {
			return metaErr
		}

		remoteChanged := remoteMetaChanged(remoteMeta, a.loadRemoteConnectionsMeta())
		localChanged := localHash != a.loadLocalConnectionsSetHash()

		if remoteMeta != nil && !remoteChanged && !localChanged {
			return a.database.SetSetting("last_sync_at", time.Now().Format(time.RFC3339))
		}

		if remoteMeta != nil && remoteChanged {
			connections, folders, err = a.mergeRemoteSyncData(connections, folders, key)
			if err != nil {
				return err
			}
			shouldUpload = localChanged
		}

		if !shouldUpload {
			if err := a.saveSyncSnapshot(connections, folders); err != nil {
				return err
			}
			if err := a.saveLocalConnectionsSetHash(syncSetHash(connections, folders)); err != nil {
				return err
			}
			if err := a.saveRemoteConnectionsMeta(remoteMeta); err != nil {
				return err
			}
			return a.database.SetSetting("last_sync_at", time.Now().Format(time.RFC3339))
		}
	} else if len(connections) > freeSyncLimit {
		connections = connections[:freeSyncLimit]
	}

	data, err := json.Marshal(connectionsSyncData{
		Connections: connections,
		Folders:     folders,
	})
	if err != nil {
		return err
	}

	meta, err := a.accountManager.SyncConnections(data, key)
	if err != nil {
		return err
	}

	if a.accountManager.GetState().Tier == "pro" {
		if err := a.saveRemoteConnectionsMeta(meta); err != nil {
			return err
		}
		if err := a.saveLocalConnectionsSetHash(syncSetHash(connections, folders)); err != nil {
			return err
		}
	}

	if err := a.saveSyncSnapshot(connections, folders); err != nil {
		return err
	}

	return a.database.SetSetting("last_sync_at", time.Now().Format(time.RFC3339))
}

// GetPublicKey returns this account's X25519 public key.
func (a *App) GetPublicKey() string {
	return a.accountManager.GetPublicKey()
}

// ShareConnection shares a connection with another Condui user by email.
func (a *App) ShareConnection(connectionID, recipientEmail string, readOnly bool, includePassword bool) error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	if !a.accountManager.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync account")
	}
	if err := a.ensureAccessTokenFresh(); err != nil {
		return fmt.Errorf("failed to refresh access token: %w", err)
	}

	conn, err := a.database.GetConnectionByID(connectionID)
	if err != nil {
		return err
	}

	shareable := *conn
	if includePassword && conn.Password != nil && *conn.Password != "" {
		password, err := decryptConnectionPassword(*conn.Password, key)
		if err != nil {
			return fmt.Errorf("failed to decrypt connection password: %w", err)
		}
		shareable.Password = &password
	} else {
		shareable.Password = nil
	}

	data, err := json.Marshal(shareable)
	if err != nil {
		return err
	}

	return a.accountManager.ShareConnection(connectionID, data, recipientEmail, readOnly, key)
}

// GetIncomingShares returns pending/accepted share invites.
func (a *App) GetIncomingShares() ([]account.ShareInfo, error) {
	if err := a.ensureAccessTokenFresh(); err != nil {
		return nil, err
	}
	return a.accountManager.GetIncomingShares()
}

// GetSentShares returns share invites created by the current user.
func (a *App) GetSentShares() ([]account.ShareInfo, error) {
	if err := a.ensureAccessTokenFresh(); err != nil {
		return nil, err
	}
	return a.accountManager.GetSentShares()
}

// CancelShare revokes an outgoing share or declines an incoming share.
func (a *App) CancelShare(shareID string) error {
	if err := a.ensureAccessTokenFresh(); err != nil {
		return err
	}
	return a.accountManager.DeleteShare(shareID)
}

// AcceptShare accepts an incoming share invite and imports the connection locally.
func (a *App) AcceptShare(shareID, encryptedKey, blobID string) error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked")
	}
	if err := a.ensureAccessTokenFresh(); err != nil {
		return fmt.Errorf("failed to refresh access token: %w", err)
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
	sharedName := "[Shared] " + conn.Name
	conn.Name = sharedName

	return a.CreateConnection(conn)
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
		SyncKey:      s.SyncKey,
	})
}

func (a *App) mergeRemoteSyncData(localConnections []models.Connection, localFolders []models.Folder, key []byte) ([]models.Connection, []models.Folder, error) {
	remoteJSON, err := a.accountManager.FetchConnections(key)
	if err != nil {
		if isMissingRemoteBlob(err) {
			return localConnections, localFolders, nil
		}
		return nil, nil, err
	}

	remote, err := parseConnectionsSyncData(remoteJSON)
	if err != nil {
		return nil, nil, err
	}

	snapshot := a.loadSyncSnapshot()
	mergedFolders, err := a.mergeRemoteFolders(localFolders, remote.Folders, snapshot)
	if err != nil {
		return nil, nil, err
	}
	mergedConnections, err := a.mergeRemoteConnections(localConnections, remote.Connections, snapshot)
	if err != nil {
		return nil, nil, err
	}

	return mergedConnections, mergedFolders, nil
}

func parseConnectionsSyncData(data []byte) (connectionsSyncData, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return connectionsSyncData{}, nil
	}

	var payload connectionsSyncData
	if err := json.Unmarshal(data, &payload); err == nil && (payload.Connections != nil || payload.Folders != nil) {
		return payload, nil
	}

	var legacy []models.Connection
	if err := json.Unmarshal(data, &legacy); err != nil {
		return connectionsSyncData{}, fmt.Errorf("invalid remote sync data: %w", err)
	}
	return connectionsSyncData{Connections: legacy}, nil
}

func (a *App) mergeRemoteFolders(local []models.Folder, remote []models.Folder, snapshot map[string]string) ([]models.Folder, error) {
	localByID := map[string]models.Folder{}
	remoteByID := map[string]models.Folder{}
	order := make([]string, 0, len(local)+len(remote))

	for _, folder := range remote {
		if folder.ID == "" {
			continue
		}
		if _, ok := remoteByID[folder.ID]; !ok {
			order = append(order, folder.ID)
		}
		remoteByID[folder.ID] = folder
	}
	for _, folder := range local {
		if folder.ID == "" {
			continue
		}
		if _, ok := remoteByID[folder.ID]; !ok {
			order = append(order, folder.ID)
		}
		localByID[folder.ID] = folder
	}

	merged := make([]models.Folder, 0, len(order))
	for _, id := range order {
		localFolder, hasLocal := localByID[id]
		remoteFolder, hasRemote := remoteByID[id]
		snapshotKey := "folder:" + id

		switch {
		case hasLocal && hasRemote:
			chosen := chooseFolder(localFolder, remoteFolder, snapshot[snapshotKey])
			if folderHash(chosen) == folderHash(remoteFolder) {
				if err := a.database.UpsertFolder(&chosen); err != nil {
					return nil, err
				}
			}
			merged = append(merged, chosen)
		case hasLocal:
			if snapshot[snapshotKey] != "" && folderHash(localFolder) == snapshot[snapshotKey] {
				if err := a.database.DeleteFolder(id); err != nil {
					return nil, err
				}
				continue
			}
			merged = append(merged, localFolder)
		case hasRemote:
			if snapshot[snapshotKey] != "" {
				continue
			}
			if err := a.database.UpsertFolder(&remoteFolder); err != nil {
				return nil, err
			}
			merged = append(merged, remoteFolder)
		}
	}

	return merged, nil
}

func (a *App) mergeRemoteConnections(local []models.Connection, remote []models.Connection, snapshot map[string]string) ([]models.Connection, error) {
	localByID := map[string]models.Connection{}
	remoteByID := map[string]models.Connection{}
	order := make([]string, 0, len(local)+len(remote))

	for _, conn := range remote {
		if conn.ID == "" {
			continue
		}
		if _, ok := remoteByID[conn.ID]; !ok {
			order = append(order, conn.ID)
		}
		remoteByID[conn.ID] = conn
	}
	for _, conn := range local {
		if conn.ID == "" {
			continue
		}
		if _, ok := remoteByID[conn.ID]; !ok {
			order = append(order, conn.ID)
		}
		localByID[conn.ID] = conn
	}

	merged := make([]models.Connection, 0, len(order))
	for _, id := range order {
		localConn, hasLocal := localByID[id]
		remoteConn, hasRemote := remoteByID[id]
		snapshotKey := "connection:" + id
		previousHash := snapshot[snapshotKey]
		if previousHash == "" {
			previousHash = snapshot[id]
		}

		switch {
		case hasLocal && hasRemote:
			chosen := chooseConnection(localConn, remoteConn, previousHash)
			if connectionHash(chosen) == connectionHash(remoteConn) {
				if err := a.database.UpsertConnection(&chosen); err != nil {
					return nil, err
				}
			}
			merged = append(merged, chosen)
		case hasLocal:
			if previousHash != "" && connectionHash(localConn) == previousHash {
				if err := a.database.DeleteConnection(id); err != nil {
					return nil, err
				}
				continue
			}
			merged = append(merged, localConn)
		case hasRemote:
			if previousHash != "" {
				continue
			}
			if err := a.database.UpsertConnection(&remoteConn); err != nil {
				return nil, err
			}
			merged = append(merged, remoteConn)
		}
	}

	return merged, nil
}

func chooseConnection(local, remote models.Connection, previousHash string) models.Connection {
	localHash := connectionHash(local)
	remoteHash := connectionHash(remote)
	if localHash == remoteHash {
		return local
	}
	if previousHash != "" && localHash == previousHash && remoteHash != previousHash {
		return remote
	}
	return local
}

func chooseFolder(local, remote models.Folder, previousHash string) models.Folder {
	localHash := folderHash(local)
	remoteHash := folderHash(remote)
	if localHash == remoteHash {
		return local
	}
	if previousHash != "" && localHash == previousHash && remoteHash != previousHash {
		return remote
	}
	return local
}

func (a *App) loadSyncSnapshot() map[string]string {
	raw, _ := a.database.GetSetting("connections_sync_snapshot")
	if raw == "" {
		return map[string]string{}
	}
	var snapshot map[string]string
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return map[string]string{}
	}
	return snapshot
}

func (a *App) saveSyncSnapshot(connections []models.Connection, folders []models.Folder) error {
	snapshot := make(map[string]string, len(connections)+len(folders))
	for _, conn := range connections {
		if conn.ID != "" {
			snapshot["connection:"+conn.ID] = connectionHash(conn)
		}
	}
	for _, folder := range folders {
		if folder.ID != "" {
			snapshot["folder:"+folder.ID] = folderHash(folder)
		}
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return a.database.SetSetting("connections_sync_snapshot", string(data))
}

func (a *App) loadRemoteConnectionsMeta() *account.BlobMeta {
	raw, _ := a.database.GetSetting("connections_remote_meta")
	if raw == "" {
		return nil
	}
	var meta account.BlobMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil
	}
	return &meta
}

func (a *App) saveRemoteConnectionsMeta(meta *account.BlobMeta) error {
	if meta == nil {
		return nil
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return a.database.SetSetting("connections_remote_meta", string(data))
}

func remoteMetaChanged(current, previous *account.BlobMeta) bool {
	if current == nil {
		return previous != nil
	}
	if previous == nil {
		return true
	}
	return current.Version != previous.Version || current.Checksum != previous.Checksum
}

func (a *App) loadLocalConnectionsSetHash() string {
	hash, _ := a.database.GetSetting("connections_local_set_hash")
	return hash
}

func (a *App) saveLocalConnectionsSetHash(hash string) error {
	return a.database.SetSetting("connections_local_set_hash", hash)
}

func syncSetHash(connections []models.Connection, folders []models.Folder) string {
	hashes := make([]string, 0, len(connections)+len(folders))
	for _, conn := range connections {
		if conn.ID != "" {
			hashes = append(hashes, "connection:"+conn.ID+":"+connectionHash(conn))
		}
	}
	for _, folder := range folders {
		if folder.ID != "" {
			hashes = append(hashes, "folder:"+folder.ID+":"+folderHash(folder))
		}
	}
	sort.Strings(hashes)
	data, _ := json.Marshal(hashes)
	sum := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func folderHash(folder models.Folder) string {
	data, _ := json.Marshal(folder)
	sum := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func connectionHash(conn models.Connection) string {
	data, _ := json.Marshal(conn)
	sum := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func isMissingRemoteBlob(err error) bool {
	return err != nil && (err.Error() == "blob not found" || err.Error() == "server error 404")
}
