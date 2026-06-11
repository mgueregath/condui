package main

import (
	"fmt"

	"ssh-gui/backend/models"
)

// GetTunnels obtiene la lista de túneles dinámicos guardados para la sesión activa
func (a *App) GetTunnels(sessionID string) ([]models.TunnelInfo, error) {
	return a.tunnelManager.List(sessionID)
}

// AddTunnel registra un nuevo túnel dinámico en la sesión actual
func (a *App) AddTunnel(sessionID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
	return a.tunnelManager.Add(sessionID, localPort, remoteHost, remotePort, a.emitLog)
}

// DeleteTunnel elimina un túnel del registro (y lo apaga si está encendido)
func (a *App) DeleteTunnel(sessionID string, tunnelID string) error {
	// Apagar primero si está corriendo
	_ = a.ToggleTunnel(sessionID, tunnelID, 0, "", 0, false)

	return a.tunnelManager.Delete(sessionID, tunnelID, a.emitLog)
}

// EditTunnel modifica los parámetros de un túnel existente
func (a *App) EditTunnel(sessionID string, tunnelID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
	// Apagar el túnel primero si estuviera corriendo en tiempo de ejecución
	_ = a.ToggleTunnel(sessionID, tunnelID, 0, "", 0, false)

	return a.tunnelManager.Edit(sessionID, tunnelID, localPort, remoteHost, remotePort, a.emitLog)
}

// ToggleTunnel enciende o apaga el túnel SSH local port forwarding de forma asíncrona
func (a *App) ToggleTunnel(sessionID string, tunnelID string, localPort int, remoteHost string, remotePort int, activate bool) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	return a.tunnelManager.Toggle(sessionID, tunnelID, localPort, remoteHost, remotePort, activate, session.Client, a.emitLog)
}
