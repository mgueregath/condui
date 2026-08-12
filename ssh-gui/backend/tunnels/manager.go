package tunnels

import (
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// Logger emits a user-facing log event.
type Logger func(logType, message, class string)

// Manager tracks the live listeners for SSH local-port-forwarding tunnels.
// Tunnel configuration itself (which tunnels exist, their host/port) is
// persisted in the database and synced across devices (see
// backend/storage/tunnels.go and models.Connection.Tunnels) — this Manager
// only owns runtime state that can't be persisted: open net.Listeners and
// the goroutines bridging them to a live ssh.Client.
type Manager struct {
	mu             sync.RWMutex
	runtimeTunnels map[string]*models.ActiveTunnel // tunnelID -> live listener
	tunnelSession  map[string]string               // tunnelID -> sessionID that opened it
}

// NewManager creates an empty tunnel manager.
func NewManager() *Manager {
	return &Manager{
		runtimeTunnels: make(map[string]*models.ActiveTunnel),
		tunnelSession:  make(map[string]string),
	}
}

// MarkActive sets Active on each tunnel based on whether it currently has a
// live listener in this process.
func (m *Manager) MarkActive(tunnels []models.TunnelInfo) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range tunnels {
		tunnels[i].Active = m.runtimeTunnels[tunnels[i].ID] != nil
	}
}

// Toggle starts or stops the local port-forwarding listener for a tunnel.
// sessionID is only needed (and client must be non-nil) when activating, so
// CloseSession can later tear it down if that session disconnects.
func (m *Manager) Toggle(sessionID, tunnelID string, localPort int, remoteHost string, remotePort int, activate bool, client *ssh.Client, log Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !activate {
		if t, exists := m.runtimeTunnels[tunnelID]; exists {
			_ = t.Listener.Close()
			delete(m.runtimeTunnels, tunnelID)
			delete(m.tunnelSession, tunnelID)
			log("TUNNEL", fmt.Sprintf("Túnel local :%d cerrado de forma segura.", t.LocalPort), "warn")
		}
		return nil
	}

	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		log("TUNNEL", fmt.Sprintf("Error abriendo puerto local :%d: %v", localPort, err), "error")
		return fmt.Errorf("no se pudo abrir el puerto local: %v", err)
	}

	m.runtimeTunnels[tunnelID] = &models.ActiveTunnel{
		Listener:   listener,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
	}
	m.tunnelSession[tunnelID] = sessionID

	log("TUNNEL", fmt.Sprintf("Túnel activo: local :%d retransmitiendo a %s:%d", localPort, remoteHost, remotePort), "success")

	// Goroutine dedicada para la escucha bidireccional del flujo TCP cifrado
	go func(l net.Listener, rHost string, rPort int) {
		for {
			localConn, err := l.Accept()
			if err != nil {
				return // Listener cerrado externamente por el Close()
			}

			// Establecer canal seguro por dentro del cliente SSH de Wails hacia la máquina remota
			remoteConn, err := client.Dial("tcp", fmt.Sprintf("%s:%d", rHost, rPort))
			if err != nil {
				localConn.Close()
				log("TUNNEL", fmt.Sprintf("Fallo de reenvío TCP hacia %s:%d", rHost, rPort), "error")
				continue
			}

			// Intercambio asíncrono bidireccional continuo (Puente simétrico)
			go func() {
				defer localConn.Close()
				defer remoteConn.Close()
				_, _ = io.Copy(localConn, remoteConn)
			}()
			go func() {
				defer localConn.Close()
				defer remoteConn.Close()
				_, _ = io.Copy(remoteConn, localConn)
			}()
		}
	}(listener, remoteHost, remotePort)

	return nil
}

// CloseSession closes every running tunnel that was activated on the given
// session, since its ssh.Client is about to become invalid.
func (m *Manager) CloseSession(sessionID string, log Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for tunnelID, owner := range m.tunnelSession {
		if owner != sessionID {
			continue
		}
		if t, exists := m.runtimeTunnels[tunnelID]; exists {
			_ = t.Listener.Close()
			delete(m.runtimeTunnels, tunnelID)
			log("TUNNEL", fmt.Sprintf("Túnel local :%d cerrado al cerrar la conexión.", t.LocalPort), "warn")
		}
		delete(m.tunnelSession, tunnelID)
	}
}
