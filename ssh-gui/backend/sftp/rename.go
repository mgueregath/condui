package sftp

import "github.com/pkg/sftp"

func RenameFile(
	client *sftp.Client,
	oldPath string,
	newPath string,
) error {

	return client.Rename(
		oldPath,
		newPath,
	)
}
