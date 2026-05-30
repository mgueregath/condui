```bash
#!/bin/bash

set -e

echo "Creando estructura Incremento 1..."

mkdir -p backend/sessions

cat > backend/sessions/session.go <<'EOF'
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

cat > backend/sessions/manager.go <<'EOF'
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

	s, ok := m.sessions[id]

	return s, ok
}

func (m *SessionManager) List() []*SSHSession {

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]*SSHSession, 0)

	for _, s := range m.sessions {
		result = append(result, s)
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

cat > backend/sessions/types.go <<'EOF'
package sessions

type ConnectionRequest struct {
	ID       string
	Host     string
	Port     int
	Username string
	Password string
}
EOF

mkdir -p frontend/src/types

cat > frontend/src/types/session.ts <<'EOF'
export interface Session {

  id: string;

  host: string;

  port: number;

  username: string;

  connected: boolean;

}
EOF

echo ""
echo "Incremento 1 preparado."
echo ""
echo "Archivos creados:"
echo "  backend/sessions/session.go"
echo "  backend/sessions/manager.go"
echo "  backend/sessions/types.go"
echo "  frontend/src/types/session.ts"
echo ""
echo "Siguiente paso:"
echo "  Integrar SessionManager dentro de app.go"
echo "  y reemplazar la sesión única actual."
```
