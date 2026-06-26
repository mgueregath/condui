package osinfo

import (
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

type OSType string

const (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSDarwin  OSType = "darwin"
	OSUnix    OSType = "unix"
	OSUnknown OSType = "unknown"
)

func DetectLocalOS() OSType {
	return FromGOOS(runtime.GOOS)
}

func FromGOOS(goos string) OSType {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows":
		return OSWindows
	case "linux":
		return OSLinux
	case "darwin":
		return OSDarwin
	case "aix", "android", "dragonfly", "freebsd", "illumos", "ios", "netbsd", "openbsd", "solaris":
		return OSUnix
	default:
		return OSUnknown
	}
}

func DetectRemoteOS(client *ssh.Client) (OSType, error) {
	if output, err := runSSHCommand(client, "cmd /c ver"); err == nil {
		value := strings.ToLower(output)
		if strings.Contains(value, "windows") || strings.Contains(value, "microsoft") {
			return OSWindows, nil
		}
	}

	output, err := runSSHCommand(client, "uname -s")
	if err == nil {
		value := strings.ToLower(strings.TrimSpace(output))

		switch {
		case strings.Contains(value, "linux"):
			return OSLinux, nil
		case strings.Contains(value, "darwin"):
			return OSDarwin, nil
		case value != "":
			return OSUnix, nil
		}
	}

	return OSUnknown, err
}

func IsWindows(osType OSType) bool {
	return osType == OSWindows
}

func IsUnixLike(osType OSType) bool {
	switch osType {
	case OSLinux, OSDarwin, OSUnix:
		return true
	default:
		return false
	}
}

func runSSHCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", err
	}

	return string(output), nil
}
