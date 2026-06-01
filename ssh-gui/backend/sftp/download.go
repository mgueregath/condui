package sftp

import (
	"io"
	"os"

	"github.com/pkg/sftp"
)

func DownloadFile(
	client *sftp.Client,
	remotePath string,
	localPath string,
) error {

	src, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)

	return err
}
