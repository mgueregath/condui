package sftp

import (
	"io"
	"os"

	"github.com/pkg/sftp"
)

func UploadFile(
	client *sftp.Client,
	localPath string,
	remotePath string,
) error {

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := client.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)

	return err
}
