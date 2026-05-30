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
