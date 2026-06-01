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
    "ssh-gui/backend/transfers"
)

type App struct {
	ctx context.Context

	sessionManager *sessions.SessionManager

	database *storage.Database

	transferManager *transfers.Manager
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
		transferManager:
    		transfers.NewManager(),

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

func (a *App) UploadFile(
	sessionID string,
	remoteDirectory string, // Ahora pasamos el directorio actual del árbol
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	localPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleccionar archivo para subir",
		Filters: []runtime.FileFilter{
			{DisplayName: "Todos los archivos", Pattern: "*.*"},
		},
	})

	if err != nil || localPath == "" {
		return fmt.Errorf("operación cancelada por el usuario")
	}

	fileName := sftpservice.GetFileName(localPath)
	
	var remotePath string
	if remoteDirectory == "/" {
		remotePath = fmt.Sprintf("/%s", fileName)
	} else {
		remotePath = fmt.Sprintf("%s/%s", remoteDirectory, fileName)
	}

	return sftpservice.UploadFile(
		session.SFTP,
		localPath,
		remotePath,
	)
}

func (a *App) DownloadFile(
	sessionID string,
	remotePath string,
	localPath string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	// 1. Abrir diálogo nativo para guardar archivo
	fileName := sftpservice.GetFileName(remotePath) // O usa path.Base(remotePath)
	
	chosenLocalPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Descargar archivo remoto",
		DefaultFilename: fileName,
	})
	
	if err != nil || chosenLocalPath == "" {
		return fmt.Errorf("descarga cancelada por el usuario")
	}

	return sftpservice.DownloadFile(
		session.SFTP,
		remotePath,
		chosenLocalPath,
	)
}

func (a *App) DeleteRemoteFile(
	sessionID string,
	path string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	return sftpservice.DeleteFile(
		session.SFTP,
		path,
	)
}

func (a *App) RenameRemoteFile(
	sessionID string,
	oldPath string,
	newPath string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	return sftpservice.RenameFile(
		session.SFTP,
		oldPath,
		newPath,
	)
}

func (a *App) CreateRemoteDirectory(
	sessionID string,
	path string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	return sftpservice.CreateDirectory(
		session.SFTP,
		path,
	)
}

func (a *App) ReadRemoteFile(
	sessionID string,
	path string,
) (string, error) {

	session, ok :=
		a.sessionManager.Get(
			sessionID,
		)

	if !ok {
		return "",
			fmt.Errorf(
				"session not found",
			)
	}

	if session.SFTP == nil {
		return "",
			fmt.Errorf(
				"sftp client nil",
			)
	}

	return sftpservice.ReadFile(
		session.SFTP,
		path,
	)
}


func (a *App) SaveRemoteFile(
	sessionID string,
	path string,
	content string,
) error {

	session, ok :=
		a.sessionManager.Get(
			sessionID,
		)

	if !ok {
		return fmt.Errorf(
			"session not found",
		)
	}

	if session.SFTP == nil {
		return fmt.Errorf(
			"sftp client nil",
		)
	}

	return sftpservice.WriteFile(
		session.SFTP,
		path,
		content,
	)
}