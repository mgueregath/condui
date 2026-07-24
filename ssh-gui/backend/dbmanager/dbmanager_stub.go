//go:build !dbmanager

package dbmanager

import (
	"io/fs"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Enabled reports whether the private db explorer submodule was compiled in.
const Enabled = false

// Manager is a no-op stand-in for the real dbexplorer.Manager.
type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func NormalizeDbType(t string) string {
	return ""
}

func (m *Manager) ConnectSQLite(client *sftp.Client, sessionID, remotePath string) (string, error) {
	return "", ErrNotAvailable
}

func (m *Manager) Connect(client *ssh.Client, sessionID, dbType string, port int, user, password, database string) (string, error) {
	return "", ErrNotAvailable
}

func (m *Manager) Disconnect(connID string) {}

func (m *Manager) ListDatabases(connID string) ([]string, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) ListSchemas(connID, database string) ([]string, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) ListTables(connID, database, schema string) ([]string, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) GetColumns(connID, database, schema, table string) ([]QueryColumn, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) Query(connID, database, query string) (QueryResult, error) {
	return QueryResult{Error: ErrNotAvailable.Error()}, ErrNotAvailable
}

func (m *Manager) GetIndexes(connID, database, schema, table string) ([]Index, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) GetForeignKeys(connID, database, schema, table string) ([]ForeignKey, error) {
	return nil, ErrNotAvailable
}

func LoadCreds() map[string]Cred {
	return map[string]Cred{}
}

func SaveCreds(creds map[string]Cred) {}

func IsKVEngine(normalizedType string) bool {
	return false
}

func (m *Manager) ConnectKV(client *ssh.Client, sessionID, dbType string, port int, user, password string) (string, error) {
	return "", ErrNotAvailable
}

func (m *Manager) KVListDatabases(connID string) ([]string, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) KVListKeys(connID, database, pattern string) ([]string, error) {
	return nil, ErrNotAvailable
}

func (m *Manager) KVGetValue(connID, database, key string) (KVValue, error) {
	return KVValue{}, ErrNotAvailable
}

func (m *Manager) KVSetValue(connID, database, key, value string) error {
	return ErrNotAvailable
}

func (m *Manager) KVDeleteKey(connID, database, key string) error {
	return ErrNotAvailable
}

func AssetsFS() (fs.FS, error) {
	return nil, ErrNotAvailable
}

func RenderIndexHTML(sessionID, dbType, portStr, remotePath, apiBase string) ([]byte, error) {
	return nil, ErrNotAvailable
}
