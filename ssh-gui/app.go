package main

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/account"
	"ssh-gui/backend/dbmanager"
	"ssh-gui/backend/sessions"
	"ssh-gui/backend/storage"
	"ssh-gui/backend/transfers"
	"ssh-gui/backend/tunnels"
)

type App struct {
	sessionManager *sessions.SessionManager

	database *storage.Database

	transferManager *transfers.Manager

	dockerLogMu       sync.Mutex
	dockerLogSessions map[string]*ssh.Session

	logServerPort int

	dbExplorer    *dbmanager.Manager
	tunnelManager *tunnels.Manager

	// Security: vault encryption
	masterKey     []byte
	syncVaultKey  []byte // Argon2(vaultPassword, fixedSalt) — same on all devices with same password
	masterKeyMu   sync.RWMutex

	// Host key verification: maps "host:port" to approval channel
	hostKeyChannels sync.Map

	// Account / sync manager
	accountManager *account.Manager

	// cancelTokenRefresh stops the background token-refresh loop
	cancelTokenRefresh context.CancelFunc

	// connectAttempts maps connectionID -> cancel func for an in-flight
	// ConnectSSH/ConnectSSHVia call, so the frontend can abort it mid-dial.
	connectAttempts sync.Map
}

func NewApp() *App {

	dbPath, err :=
		storage.DatabasePath()

	if err != nil {
		panic(err)
	}

	db, err :=
		storage.NewDatabase(
			dbPath,
		)

	if err != nil {
		panic(err)
	}

	if err := db.Migrate(); err != nil {
		panic(err)
	}

	am := account.NewManager()

	// Restore saved account state from DB
	if row, _ := db.GetAccount(); row != nil {
		am.Load(account.State{
			ServerURL:    row.ServerURL,
			AccessToken:  row.AccessToken,
			RefreshToken: row.RefreshToken,
			UserID:       row.UserID,
			Email:        row.Email,
			Tier:         row.Tier,
			PublicKey:    row.PublicKey,
			IdentityBlob: row.IdentityBlob,
			SyncKey:      row.SyncKey,
		})
	}

	return &App{
		sessionManager:  sessions.NewSessionManager(),
		transferManager: transfers.NewManager(),
		dbExplorer:      dbmanager.NewManager(),
		tunnelManager:   tunnels.NewManager(),
		accountManager:  am,
		database:        db,
	}
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	go a.startDockerLogServer()
	a.startTokenRefreshLoop(ctx)
	return nil
}

// GetFeatures reports which build-time-only features were compiled into this
// binary, so the frontend can hide UI for capabilities that aren't present
// (e.g. the private db manager submodule).
func (a *App) GetFeatures() map[string]bool {
	return map[string]bool{
		"dbManager": dbmanager.Enabled,
	}
}

// startTokenRefreshLoop refreshes the access token (and tier) on startup and
// every few minutes while the app is running. Server access tokens default to
// 15 minutes, so a 24h cadence leaves the frontend holding expired tokens.
func (a *App) startTokenRefreshLoop(parentCtx context.Context) {
	loopCtx, cancel := context.WithCancel(parentCtx)
	a.cancelTokenRefresh = cancel

	go func() {
		// Short delay so the UI is ready before we hit the network
		select {
		case <-time.After(3 * time.Second):
		case <-loopCtx.Done():
			return
		}

		a.doTokenRefresh()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.doTokenRefresh()
			case <-loopCtx.Done():
				return
			}
		}
	}()
}

func (a *App) ensureAccessTokenFresh() error {
	if !a.accountManager.IsLoggedIn() {
		return nil
	}
	if !a.accountManager.AccessTokenStale(2 * time.Minute) {
		return nil
	}
	updated, err := a.accountManager.RefreshAccessToken()
	if err != nil {
		return err
	}
	return a.saveAccountState(updated)
}

// doTokenRefresh refreshes the access token and persists the updated state.
// It is a no-op if no account is logged in.
func (a *App) doTokenRefresh() {
	if !a.accountManager.IsLoggedIn() {
		return
	}
	updated, err := a.accountManager.RefreshAccessToken()
	if err != nil {
		log.Printf("[account] token refresh failed: %v", err)
		return
	}
	if err := a.saveAccountState(updated); err != nil {
		log.Printf("[account] failed to persist refreshed state: %v", err)
	}
}

func (a *App) emitLog(logType string, message string, class string) {
	application.Get().Event.Emit("log-event", map[string]string{
		"time": time.Now().Format("15:04:05"),
		"type": logType,
		"msg":  message,
		"cls":  class, // "success", "warn", "error" o ""
	})
}

// getMasterKey returns the in-memory master key, or nil if vault is locked.
func (a *App) getMasterKey() []byte {
	a.masterKeyMu.RLock()
	defer a.masterKeyMu.RUnlock()
	return a.masterKey
}

// setMasterKey stores the derived master key in memory.
func (a *App) setMasterKey(key []byte) {
	a.masterKeyMu.Lock()
	defer a.masterKeyMu.Unlock()
	a.masterKey = key
}

// clearMasterKey wipes the master key from memory.
func (a *App) clearMasterKey() {
	a.masterKeyMu.Lock()
	defer a.masterKeyMu.Unlock()
	for i := range a.masterKey {
		a.masterKey[i] = 0
	}
	a.masterKey = nil
}

func (a *App) getSyncVaultKey() []byte {
	a.masterKeyMu.RLock()
	defer a.masterKeyMu.RUnlock()
	return a.syncVaultKey
}

func (a *App) setSyncVaultKey(key []byte) {
	a.masterKeyMu.Lock()
	defer a.masterKeyMu.Unlock()
	a.syncVaultKey = key
}

func (a *App) clearSyncVaultKey() {
	a.masterKeyMu.Lock()
	defer a.masterKeyMu.Unlock()
	for i := range a.syncVaultKey {
		a.syncVaultKey[i] = 0
	}
	a.syncVaultKey = nil
}

// approveHostKeyChannel sends a boolean on the channel for "host:port".
func (a *App) approveHostKeyChannel(key string, approved bool) {
	if ch, ok := a.hostKeyChannels.Load(key); ok {
		ch.(chan bool) <- approved
	}
}

// registerConnectAttempt records the cancel func for an in-flight connection
// attempt so CancelConnect can abort it.
func (a *App) registerConnectAttempt(connectionID string, cancel context.CancelFunc) {
	a.connectAttempts.Store(connectionID, cancel)
}

// clearConnectAttempt removes a connection attempt's cancel func once it has
// finished (successfully, with an error, or canceled).
func (a *App) clearConnectAttempt(connectionID string) {
	a.connectAttempts.Delete(connectionID)
}

// CancelConnect aborts an in-flight ConnectSSH/ConnectSSHVia call for the
// given connection, if one is currently in progress.
func (a *App) CancelConnect(connectionID string) {
	if v, ok := a.connectAttempts.Load(connectionID); ok {
		if cancel, ok := v.(context.CancelFunc); ok {
			cancel()
		}
	}
}
