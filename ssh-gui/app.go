package main

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/account"
	"ssh-gui/backend/dbexplorer"
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

	dbExplorer    *dbexplorer.Manager
	tunnelManager *tunnels.Manager

	// Security: vault encryption
	masterKey     []byte
	masterKeyMu   sync.RWMutex

	// Host key verification: maps "host:port" to approval channel
	hostKeyChannels sync.Map

	// Account / sync manager
	accountManager *account.Manager

	// cancelTokenRefresh stops the background token-refresh loop
	cancelTokenRefresh context.CancelFunc
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
		})
	}

	return &App{
		sessionManager:  sessions.NewSessionManager(),
		transferManager: transfers.NewManager(),
		dbExplorer:      dbexplorer.NewManager(),
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

// startTokenRefreshLoop refreshes the access token (and tier) on startup and
// once every 24 hours while the app is running.
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

		ticker := time.NewTicker(24 * time.Hour)
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

// approveHostKeyChannel sends a boolean on the channel for "host:port".
func (a *App) approveHostKeyChannel(key string, approved bool) {
	if ch, ok := a.hostKeyChannels.Load(key); ok {
		ch.(chan bool) <- approved
	}
}
