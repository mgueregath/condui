package sftp

import "github.com/pkg/sftp"

func CreateDirectory(
	client *sftp.Client,
	path string,
) error {

	return client.Mkdir(path)
}
