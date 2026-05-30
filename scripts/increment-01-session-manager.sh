#!/bin/bash

set -e

echo "========================================"
echo "Incremento 1 - Session Manager"
echo "========================================"

mkdir -p ssh-gui/backend/sessions
mkdir -p ssh-gui/frontend/src/types

cat > ssh-gui/backend/sessions/session.go <<'EOF'
package sessions

import (
	"io"

	"golang.org/x/crypto/ssh"
)

type SSHSession struct {
	ID string

	Client  *ssh.Client
	Session *ssh.Session

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	Connected bool

	Rows int
	Cols int
}
EOF

cat > ssh-gui/backend/sessions/manager.go <<'EOF'
package sessions

import (
	"errors"
	"sync"
)

type SessionManager struct {
	mutex    sync.RWMutex
	sessions map[string]*SSHSession
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*SSHSession),
	}
}

func (m *SessionManager) Add(session *SSHSession) {

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.sessions[session.ID] = session
}

func (m *SessionManager) Get(id string) (*SSHSession, bool) {

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	session, ok := m.sessions[id]

	return session, ok
}

func (m *SessionManager) List() []*SSHSession {

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]*SSHSession, 0)

	for _, session := range m.sessions {
		result = append(result, session)
	}

	return result
}

func (m *SessionManager) Remove(id string) error {

	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, ok := m.sessions[id]

	if !ok {
		return errors.New("session not found")
	}

	if session.Session != nil {
		_ = session.Session.Close()
	}

	if session.Client != nil {
		_ = session.Client.Close()
	}

	delete(m.sessions, id)

	return nil
}
EOF

cat > ssh-gui/backend/sessions/types.go <<'EOF'
package sessions

type ConnectionRequest struct {
	ID       string
	Host     string
	Port     int
	Username string
	Password string
}
EOF

cat > ssh-gui/frontend/src/types/session.ts <<'EOF'
export interface Session {
  id: string;
  host: string;
  port: number;
  username: string;
  connected: boolean;
}
EOF

echo ""
echo "Archivos generados:"
echo ""
echo "ssh-gui/backend/sessions/session.go"
echo "ssh-gui/backend/sessions/manager.go"
echo "ssh-gui/backend/sessions/types.go"
echo "ssh-gui/frontend/src/types/session.ts"
echo ""
echo "Incremento 1 completado."