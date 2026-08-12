package main

import (
	"fmt"

	"ssh-gui/backend/models"
)

// GetTunnels returns the tunnels persisted for a connection, with Active
// reflecting whether each currently has a live listener in this process.
func (a *App) GetTunnels(connectionID string) ([]models.TunnelInfo, error) {
	tunnels, err := a.database.GetTunnelsByConnectionID(connectionID)
	if err != nil {
		return nil, err
	}
	a.tunnelManager.MarkActive(tunnels)
	return tunnels, nil
}

// AddTunnel persists a new tunnel for a connection so it survives
// reconnects/restarts and syncs to the user's other devices.
func (a *App) AddTunnel(connectionID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
	tunnel, err := a.database.CreateTunnel(connectionID, localPort, remoteHost, remotePort)
	if err != nil {
		return models.TunnelInfo{}, err
	}
	a.triggerBackgroundSync()
	return tunnel, nil
}

// DeleteTunnel removes a persisted tunnel, turning it off first if it has a
// live listener running.
func (a *App) DeleteTunnel(tunnelID string) error {
	_ = a.tunnelManager.Toggle("", tunnelID, 0, "", 0, false, nil, a.emitLog)

	if err := a.database.DeleteTunnel(tunnelID); err != nil {
		return err
	}
	a.triggerBackgroundSync()
	return nil
}

// EditTunnel updates a persisted tunnel's parameters, turning it off first
// if it has a live listener running (its old host/port would otherwise keep
// forwarding after the edit).
func (a *App) EditTunnel(tunnelID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
	_ = a.tunnelManager.Toggle("", tunnelID, 0, "", 0, false, nil, a.emitLog)

	tunnel, err := a.database.UpdateTunnel(tunnelID, localPort, remoteHost, remotePort)
	if err != nil {
		return models.TunnelInfo{}, err
	}
	a.triggerBackgroundSync()
	return tunnel, nil
}

// ToggleTunnel starts or stops the local port-forwarding listener for a
// tunnel on the given live SSH session.
func (a *App) ToggleTunnel(sessionID string, tunnelID string, localPort int, remoteHost string, remotePort int, activate bool) error {
	if !activate {
		return a.tunnelManager.Toggle(sessionID, tunnelID, localPort, remoteHost, remotePort, false, nil, a.emitLog)
	}

	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	return a.tunnelManager.Toggle(sessionID, tunnelID, localPort, remoteHost, remotePort, true, session.Client, a.emitLog)
}
