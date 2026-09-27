package sftp

import (
	"fmt"
	"path"

	"github.com/pkg/sftp"
)

func DeleteFile(client *sftp.Client, remotePath string) error {
	if client == nil {
		return fmt.Errorf("sftp client nil")
	}
	cleanPath := path.Clean(remotePath)
	if cleanPath == "." || cleanPath == "/" || cleanPath == ".." {
		return fmt.Errorf("invalid deletion path: %s", remotePath)
	}
	return deleteEntry(client, cleanPath)
}

func deleteEntry(client *sftp.Client, remotePath string) error {
	// Lstat ensures symbolic links are removed without following their targets.
	info, err := client.Lstat(remotePath)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", remotePath, err)
	}
	if !info.IsDir() {
		if err := client.Remove(remotePath); err != nil {
			return fmt.Errorf("remove %s: %w", remotePath, err)
		}
		return nil
	}

	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", remotePath, err)
	}
	for _, entry := range entries {
		if err := deleteEntry(client, path.Join(remotePath, entry.Name())); err != nil {
			return err
		}
	}
	if err := client.RemoveDirectory(remotePath); err != nil {
		return fmt.Errorf("remove directory %s: %w", remotePath, err)
	}
	return nil
}
