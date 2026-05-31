package main

import (
	"fmt"
	"context"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-gui/backend/sessions"
    "ssh-gui/backend/storage"
	"ssh-gui/backend/models"

	sftpservice "ssh-gui/backend/sftp"
)

type App struct {
	ctx context.Context

	sessionManager *sessions.SessionManager

	database *storage.Database
}

func NewApp() *App {

	db, err :=
		storage.NewDatabase(
			"modernterm.db",
		)

	if err != nil {
		panic(err)
	}

	if err := db.Migrate(); err != nil {
		panic(err)
	}

	return &App{
		sessionManager:
			sessions.NewSessionManager(),

		database: db,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) ConnectSSH(
	connectionID string,
) (string, error) {

	connection, err :=
		a.database.GetConnectionByID(
			connectionID,
		)

	if err != nil {
		return "", err
	}

	config := &ssh.ClientConfig{
	User: connection.Username,
	Auth: []ssh.AuthMethod{
		ssh.Password(
			*connection.Password,
		),
	},
	HostKeyCallback:
		ssh.InsecureIgnoreHostKey(),
}


	client, err := ssh.Dial(
	"tcp",
	fmt.Sprintf(
		"%s:%d",
		connection.Host,
		connection.Port,
	),
	config,
)

	if err != nil {
		return "", err
	}

	sftpClient, err := sftp.NewClient(client)

if err != nil {
    return "", err
}

	session, err := client.NewSession()

	if err != nil {
		return "", err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	err = session.RequestPty(
		"xterm",
		40,
		120,
		modes,
	)

	if err != nil {
		return "", err
	}

	stdin, err := session.StdinPipe()

	if err != nil {
		return "", err
	}

	stdout, err := session.StdoutPipe()

	if err != nil {
		return "", err
	}

	stderr, err := session.StderrPipe()

	if err != nil {
		return "", err
	}

	sessionID := uuid.NewString()

	a.sessionManager.Add(
		&sessions.SSHSession{
			ID: sessionID,

			Client: client,
			Session: session,

        	SFTP: sftpClient,

			Stdin: stdin,
			Stdout: stdout,
			Stderr: stderr,

			Connected: true,

			Rows: 40,
			Cols: 120,
		},
	)

	if err := session.Shell(); err != nil {
		return "", err
	}

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stdout.Read(buffer)

			if err != nil {
				return
			}

			runtime.EventsEmit(
				a.ctx,
				"terminal-output",
				map[string]any{
					"sessionId": sessionID,
					"data":      string(buffer[:n]),
				},
			)
		}
	}()

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stderr.Read(buffer)

			if err != nil {
				return
			}

			runtime.EventsEmit(
				a.ctx,
				"terminal-output",
				map[string]any{
					"sessionId": sessionID,
					"data":      string(buffer[:n]),
				},
			)
		}
	}()

	return sessionID, nil
}

func (a *App) SendInput(
	sessionID string,
	data string,
) {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return
	}

	_, _ = session.Stdin.Write(
		[]byte(data),
	)
}

func (a *App) ListSessions() []string {

	list :=
		a.sessionManager.List()

	result := []string{}

	for _, s := range list {
		result = append(result, s.ID)
	}

	return result
}

func (a *App) CloseSession(
	sessionID string,
) error {

	return a.sessionManager.Remove(
		sessionID,
	)
}

func (a *App) ResizeTerminal(
	sessionID string,
	rows int,
	cols int,
) {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return
	}

	if session.Session == nil {
		return
	}

	_ = session.Session.WindowChange(
		rows,
		cols,
	)

	session.Rows = rows
	session.Cols = cols
}

func (a *App) CreateFolder(
	name string,
) error {

	_, err :=
		a.database.CreateFolder(
			name,
		)

	return err
}

func (a *App) GetFolders() (
	[]models.Folder,
	error,
) {

	return a.database.GetFolders()

}

func (a *App) GetConnections() (
	[]models.Connection,
	error,
) {

	return a.database.GetConnections()

}

func (a *App) CreateConnection(
	connection models.Connection,
) error {

	return a.database.CreateConnection(
		&connection,
	)
}

func (a *App) UpdateConnection(
	connection models.Connection,
) error {

	return a.database.UpdateConnection(
		&connection,
	)
}

func (a *App) DeleteConnection(
	id string,
) error {

	return a.database.DeleteConnection(
		id,
	)
}

func (a *App) UpdateFolder(
	id string,
	name string,
) error {

	return a.database.UpdateFolder(
		id,
		name,
	)
}

func (a *App) DeleteFolder(
	id string,
) error {

	return a.database.DeleteFolder(
		id,
	)
}

func (a *App) GetConnectionByID(
	id string,
) (*models.Connection, error) {

	return a.database.GetConnectionByID(
		id,
	)

}

func (a *App) ListDirectory(
    sessionID string,
    path string,
) (
    []sftpservice.FileItem,
    error,
) {

    session, ok :=
        a.sessionManager.Get(
            sessionID,
        )

    if !ok {

        return nil,
            fmt.Errorf(
                "session not found",
            )

    }

    if session.SFTP == nil {

        return nil,
            fmt.Errorf(
                "sftp client nil",
            )

    }

    return sftpservice.ListDirectory(
        session.SFTP,
        path,
    )
}