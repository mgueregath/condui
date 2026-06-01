package sftp

import (
	"io"

	pkgsftp "github.com/pkg/sftp"
)

func ReadFile(
	client *pkgsftp.Client,
	path string,
) (string, error) {

	file, err :=
		client.Open(path)

	if err != nil {
		return "", err
	}

	defer file.Close()


	data, err :=
		io.ReadAll(file)

	if err != nil {
		return "", err
	}


	return string(data), nil
}