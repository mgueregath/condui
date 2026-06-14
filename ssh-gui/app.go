package main

import (
	"context"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v3/pkg/application"

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

	return &App{
		sessionManager:  sessions.NewSessionManager(),
		transferManager: transfers.NewManager(),
		dbExplorer:      dbexplorer.NewManager(),
		tunnelManager:   tunnels.NewManager(),

		database: db,
	}
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	go a.startDockerLogServer()
	return nil
}

func (a *App) emitLog(logType string, message string, class string) {
	application.Get().Event.Emit("log-event", map[string]string{
		"time": time.Now().Format("15:04:05"),
		"type": logType,
		"msg":  message,
		"cls":  class, // "success", "warn", "error" o ""
	})
}
