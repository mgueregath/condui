package sftp

import (
	"path"
	"strings"

	"ssh-gui/backend/osinfo"
)

func GetFileName(remotePath string) string {
	return GetRemoteFileName(remotePath, osinfo.OSUnknown)
}

func GetRemoteFileName(remotePath string, remoteOS osinfo.OSType) string {
	if osinfo.IsWindows(remoteOS) {
		remotePath = strings.ReplaceAll(remotePath, `\`, `/`)
	}

	return path.Base(remotePath)
}
