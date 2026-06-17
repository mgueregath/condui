package main

import (
	"fmt"

	"ssh-gui/backend/models"
	"ssh-gui/backend/vboxinfo"
)

// GetVirtualBoxVMs lists all VMs on the remote host.
// Returns an error if VBoxManage is not installed.
func (a *App) GetVirtualBoxVMs(sessionID string) ([]models.VMInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return vboxinfo.GetVMs(session.Client)
}

// IsVirtualBoxAvailable checks whether VBoxManage is installed on the remote host.
func (a *App) IsVirtualBoxAvailable(sessionID string) bool {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return false
	}
	return vboxinfo.IsAvailable(session.Client)
}

// VirtualBoxAction performs a lifecycle action on a VM.
// Valid actions: start-gui, start-headless, stop-acpi, stop-force,
// pause, resume, reset, savestate.
func (a *App) VirtualBoxAction(sessionID, vmName, action string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found")
	}

	out, err := vboxinfo.ControlVM(session.Client, vmName, action)
	if err == nil {
		a.emitLog("VBOX", fmt.Sprintf("VM \"%s\" → %s", vmName, action), "success")
	} else {
		a.emitLog("VBOX", fmt.Sprintf("Error VM \"%s\" (%s): %s", vmName, action, out), "error")
	}
	return out, err
}
