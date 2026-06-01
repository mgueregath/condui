package sftp

import "github.com/pkg/sftp"

func DeleteFile(
	client *sftp.Client,
	path string,
) error {

	return client.Remove(path)
}
