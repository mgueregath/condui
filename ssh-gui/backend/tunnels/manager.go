package tunnels

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// Logger emits a user-facing log event.
type Logger func(logType, message, class string)

// Manager tracks registered and running SSH local-port-forwarding tunnels.
type Manager struct {
	mu                sync.RWMutex
	runtimeTunnels    map[string]*models.ActiveTunnel
	registeredTunnels map[string][]models.TunnelInfo
}

// NewManager creates an empty tunnel manager.
func NewManager() *Manager {
	return &Manager{
		runtimeTunnels:    make(map[string]*models.ActiveTunnel),
		registeredTunnels: make(map[string][]models.TunnelInfo),
	}
}

// List returns the tunnels registered for the given session, with their live Active state.
func (m *Manager) List(sessionID string) ([]models.TunnelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list, exists := m.registeredTunnels[sessionID]
	if !exists {
		return []models.TunnelInfo{}, nil
	}

	// Sincronizar el flag de Active en tiempo real basándose en si el listener genérico sigue vivo
	for i := range list {
		list[i].Active = m.runtimeTunnels[list[i].ID] != nil
	}

	return list, nil
}

// Add registers a new dynamic tunnel for the given session.
func (m *Manager) Add(sessionID string, localPort int, remoteHost string, remotePort int, log Logger) (models.TunnelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnelID := uuid.NewString()
	newTunnel := models.TunnelInfo{
		ID:         tunnelID,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Active:     false,
	}

	m.registeredTunnels[sessionID] = append(m.registeredTunnels[sessionID], newTunnel)
	log("TUNNEL", fmt.Sprintf("Nuevo túnel registrado: :%d -> %s:%d", localPort, remoteHost, remotePort), "")

	return newTunnel, nil
}

// Delete removes a tunnel from the registry. Callers should Toggle it off first.
func (m *Manager) Delete(sessionID, tunnelID string, log Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	list := m.registeredTunnels[sessionID]
	for i, t := range list {
		if t.ID == tunnelID {
			m.registeredTunnels[sessionID] = append(list[:i], list[i+1:]...)
			log("TUNNEL", fmt.Sprintf("Túnel :%d eliminado del registro.", t.LocalPort), "warn")
			break
		}
	}
	return nil
}

// Edit updates the parameters of an existing tunnel. Callers should Toggle it off first.
func (m *Manager) Edit(sessionID, tunnelID string, localPort int, remoteHost string, remotePort int, log Logger) (models.TunnelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	list, exists := m.registeredTunnels[sessionID]
	if !exists {
		return models.TunnelInfo{}, fmt.Errorf("no se encontraron túneles para esta sesión")
	}

	for i, t := range list {
		if t.ID == tunnelID {
			list[i].LocalPort = localPort
			list[i].RemoteHost = remoteHost
			list[i].RemotePort = remotePort

			m.registeredTunnels[sessionID] = list
			log("TUNNEL", fmt.Sprintf("Túnel editado con éxito: Mapeado a :%d", localPort), "success")
			return list[i], nil
		}
	}

	return models.TunnelInfo{}, fmt.Errorf("túnel no encontrado")
}

// Toggle starts or stops the local port-forwarding listener for a tunnel.
func (m *Manager) Toggle(sessionID, tunnelID string, localPort int, remoteHost string, remotePort int, activate bool, client *ssh.Client, log Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !activate {
		// APAGAR TÚNEL
		if t, exists := m.runtimeTunnels[tunnelID]; exists {
			t.Listener.Close()
			delete(m.runtimeTunnels, tunnelID)
			log("TUNNEL", fmt.Sprintf("Túnel local :%d cerrado de forma segura.", t.LocalPort), "warn")
		}
		return nil
	}

	// Si se activa por interfaz, pero no se pasaron parámetros directos, buscamos en el registro guardado
	if localPort == 0 {
		for _, t := range m.registeredTunnels[sessionID] {
			if t.ID == tunnelID {
				localPort = t.LocalPort
				remoteHost = t.RemoteHost
				remotePort = t.RemotePort
				break
			}
		}
	}

	// ENCENDER TÚNEL
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
