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
