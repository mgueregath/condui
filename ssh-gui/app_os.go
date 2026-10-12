package main

import (
	"fmt"

	"ssh-gui/backend/osinfo"
)

func (a *App) DetectLocalOS() osinfo.OSType {
	return osinfo.DetectLocalOS()
}

func (a *App) DetectRemoteOS(sessionID string) (osinfo.OSType, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return osinfo.OSUnknown, fmt.Errorf("session not found")
	}

	if session.RemoteOS != "" && session.RemoteOS != osinfo.OSUnknown {
		return session.RemoteOS, nil
	}

	remoteOS, err := osinfo.DetectRemoteOS(session.Client)
	if err != nil {
		return osinfo.OSUnknown, err
	}

	session.RemoteOS = remoteOS

	return remoteOS, nil
}

// DetectRemoteLinuxDistribution returns distribution metadata for a Linux SSH host.
func (a *App) DetectRemoteLinuxDistribution(sessionID string) (osinfo.LinuxDistribution, error) {
	remoteOS, err := a.DetectRemoteOS(sessionID)
	if err != nil {
		return osinfo.LinuxDistribution{}, err
	}
	if remoteOS != osinfo.OSLinux {
		return osinfo.LinuxDistribution{}, fmt.Errorf("remote host is not Linux")
	}
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return osinfo.LinuxDistribution{}, fmt.Errorf("session not found")
	}
	return osinfo.DetectRemoteLinuxDistribution(session.Client)
}
