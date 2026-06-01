package sftp

import (
	pkgsftp "github.com/pkg/sftp"
)

func WriteFile(
	client *pkgsftp.Client,
	path string,
	content string,
) error {

	file, err :=
		client.Create(
			path,
		)

	if err != nil {
		return err
	}

	defer file.Close()


	_,
	err =
		file.Write(
			[]byte(content),
		)


	return err
}