package sftp

import (
	"fmt"
	"path"

	"github.com/pkg/sftp"
)

func DeleteFile(client *sftp.Client, remotePath string) error {
	return DeleteFileWithProgress(client, remotePath, nil)
}

func DeleteFileWithProgress(client *sftp.Client, remotePath string, progress func(int, int)) error {
	if client == nil {
		return fmt.Errorf("sftp client nil")
	}
	cleanPath := path.Clean(remotePath)
	if cleanPath == "." || cleanPath == "/" || cleanPath == ".." {
		return fmt.Errorf("invalid deletion path: %s", remotePath)
	}
	total, err := countDeleteEntries(client, cleanPath)
	if err != nil {
		return err
	}
	completed := 0
	if progress != nil {
		progress(completed, total)
	}
	return deleteEntry(client, cleanPath, func() {
		completed++
		if progress != nil {
			progress(completed, total)
		}
	})
}

func countDeleteEntries(client *sftp.Client, remotePath string) (int, error) {
	info, err := client.Lstat(remotePath)
	if err != nil {
		return 0, fmt.Errorf("inspect %s: %w", remotePath, err)
	}
	total := 1
	if !info.IsDir() {
		return total, nil
	}
	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return 0, fmt.Errorf("read directory %s: %w", remotePath, err)
	}
	for _, entry := range entries {
		count, err := countDeleteEntries(client, path.Join(remotePath, entry.Name()))
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func deleteEntry(client *sftp.Client, remotePath string, removed func()) error {
	// Lstat ensures symbolic links are removed without following their targets.
	info, err := client.Lstat(remotePath)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", remotePath, err)
	}
	if !info.IsDir() {
		if err := client.Remove(remotePath); err != nil {
			return fmt.Errorf("remove %s: %w", remotePath, err)
		}
		removed()
		return nil
	}

	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", remotePath, err)
	}
	for _, entry := range entries {
		if err := deleteEntry(client, path.Join(remotePath, entry.Name()), removed); err != nil {
			return err
		}
	}
	if err := client.RemoveDirectory(remotePath); err != nil {
		return fmt.Errorf("remove directory %s: %w", remotePath, err)
	}
	removed()
	return nil
}
