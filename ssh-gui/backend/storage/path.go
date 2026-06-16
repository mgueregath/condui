package storage

import (
	"os"
	"path/filepath"
)


func DatabasePath() (string, error) {

	configDir, err :=
		os.UserConfigDir()

	if err != nil {
		return "", err
	}


	appDir :=
		filepath.Join(
			configDir,
			"Condui",
		)


	err =
		os.MkdirAll(
			appDir,
			0700,
		)

	if err != nil {
		return "", err
	}


	return filepath.Join(
		appDir,
		"condui.db",
	), nil
}