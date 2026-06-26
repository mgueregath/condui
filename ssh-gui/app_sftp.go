package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/models"
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
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	localPath, err := application.Get().Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "Subir archivo",
	}).PromptForSingleSelection()
	if err != nil || localPath == "" {
		return fmt.Errorf("cancelado")
	}

	fileName := filepath.Base(localPath)
	var remotePath string
	if remoteDirectory == "/" {
		remotePath = fmt.Sprintf("/%s", fileName)
	} else {
		remotePath = fmt.Sprintf("%s/%s", remoteDirectory, fileName)
	}

	// Abrir archivo local para obtener su peso total
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return err
	}

	// Crear archivo en el servidor remoto por SFTP
	remoteFile, err := session.SFTP.Create(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	transferID := uuid.NewString()
	a.emitLog("SFTP", "Iniciando subida de: "+fileName, "")

	// Envolver el destino en nuestro ProgressWriter
	progress := &models.ProgressWriter{
		Total:     stat.Size(),
		ID:        transferID,
		FileName:  fileName,
		Direction: "upload",
	}

	// MultiWriter escribe en el archivo remoto y a la vez computa el progreso
	mw := io.MultiWriter(remoteFile, progress)
	_, err = io.Copy(mw, localFile)

	if err != nil {
		application.Get().Event.Emit("transfer-status", map[string]any{"id": transferID, "status": "error"})
		a.emitLog("SFTP", "Error al subir "+fileName, "error")
		return err
	}

	application.Get().Event.Emit("transfer-status", map[string]any{"id": transferID, "name": fileName, "progress": 100, "status": "done"})
	a.emitLog("SFTP", "Subida completada: "+fileName, "success")
	return nil
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
	fileName := sftpservice.GetRemoteFileName(remotePath, session.RemoteOS)

	chosenLocalPath, err := application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Descargar archivo remoto",
		Filename: fileName,
	}).PromptForSingleSelection()
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
