package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	sftpservice "ssh-gui/backend/sftp"
)

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

func (a *App) UploadFile(sessionID string, remoteDirectory string) error {
	localPath, err := application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "Subir archivo",
	}).PromptForSingleSelection()
	if err != nil || localPath == "" {
		return fmt.Errorf("cancelado")
	}

	return a.uploadLocalPath(sessionID, remoteDirectory, localPath)
}

func (a *App) UploadDroppedFile(sessionID string, remoteDirectory string, localPath string) error {
	if strings.TrimSpace(localPath) == "" {
		return fmt.Errorf("local path required")
	}

	return a.uploadLocalPath(sessionID, remoteDirectory, localPath)
}

func (a *App) uploadLocalFile(sessionID string, remoteDirectory string, localPath string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.SFTP == nil {
		return fmt.Errorf("sftp client nil")
	}

	fileName := filepath.Base(localPath)
	if fileName == "." || fileName == string(filepath.Separator) {
		return fmt.Errorf("invalid file path")
	}

	var remotePath string
	if remoteDirectory == "/" {
		remotePath = fmt.Sprintf("/%s", fileName)
	} else {
		remotePath = fmt.Sprintf("%s/%s", remoteDirectory, fileName)
	}

	return a.runFileTransfer(fileName, "upload", func(ctx context.Context, progress func(int64) io.Writer) error {
		return sftpservice.UploadFile(ctx, session.SFTP, localPath, remotePath, progress)
	})
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

	if session.SFTP == nil {
		return fmt.Errorf("sftp client nil")
	}

	// 1. Abrir diálogo nativo para guardar archivo
	fileName := sftpservice.GetRemoteFileName(remotePath, session.RemoteOS)

	chosenLocalPath, err := application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Descargar archivo remoto",
		Filename: fileName,
	}).PromptForSingleSelection()
	if err != nil || chosenLocalPath == "" {
		return fmt.Errorf("descarga cancelada por el usuario")
	}

	return a.runFileTransfer(fileName, "download", func(ctx context.Context, progress func(int64) io.Writer) error {
		return sftpservice.DownloadFile(ctx, session.SFTP, remotePath, chosenLocalPath, progress)
	})
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

	content, err :=
		sftpservice.ReadFile(
			session.SFTP,
			path,
		)

	if err != nil {
		return "", err
	}

	ext :=
		strings.ToLower(
			filepath.Ext(path),
		)

	switch ext {

	case ".png",
		".jpg",
		".jpeg",
		".gif",
		".webp",
		".bmp":

		return base64.StdEncoding.EncodeToString(
			[]byte(content),
		), nil

	default:

		return content, nil

	}

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
