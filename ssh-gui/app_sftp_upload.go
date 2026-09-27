package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// uploadLocalPath preserves the selected directory and its contents on the host.
func (a *App) uploadLocalPath(sessionID, remoteDirectory, localPath string) error {
	localPath = filepath.Clean(localPath)
	name := filepath.Base(localPath)
	if name == "." || name == ".." || name == string(filepath.Separator) || localPath == filepath.VolumeName(localPath)+string(filepath.Separator) {
		return fmt.Errorf("invalid upload path: %s", localPath)
	}

	info, err := os.Lstat(localPath)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", localPath, err)
	}
	if info.Mode().IsRegular() {
		if err := a.uploadLocalFile(sessionID, remoteDirectory, localPath); err != nil {
			return fmt.Errorf("upload %s: %w", localPath, err)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("unsupported upload entry (symbolic link or special file): %s", localPath)
	}

	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session.SFTP == nil {
		return fmt.Errorf("sftp client nil")
	}

	entries, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", localPath, err)
	}
	remotePath := path.Join(remoteDirectory, name)
	if err := session.SFTP.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("create remote directory %s: %w", remotePath, err)
	}
	for _, entry := range entries {
		if err := a.uploadLocalPath(sessionID, remotePath, filepath.Join(localPath, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
