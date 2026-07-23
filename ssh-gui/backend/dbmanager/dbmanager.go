//go:build dbmanager

package dbmanager

import (
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	dbexplorer "github.com/mgueregath/navoro"
)

// Enabled reports whether the private db explorer submodule was compiled in.
const Enabled = true

// Manager wraps the private dbexplorer.Manager.
type Manager struct {
	impl *dbexplorer.Manager
}

func NewManager() *Manager {
	return &Manager{impl: dbexplorer.NewManager()}
}

func NormalizeDbType(t string) string {
	return dbexplorer.NormalizeDbType(t)
}

func (m *Manager) ConnectSQLite(client *sftp.Client, sessionID, remotePath string) (string, error) {
	return m.impl.ConnectSQLite(client, sessionID, remotePath)
}

func (m *Manager) Connect(client *ssh.Client, sessionID, dbType string, port int, user, password string) (string, error) {
	return m.impl.Connect(client, sessionID, dbType, port, user, password)
}

func (m *Manager) Disconnect(connID string) {
	m.impl.Disconnect(connID)
}

func (m *Manager) ListDatabases(connID string) ([]string, error) {
	return m.impl.ListDatabases(connID)
}

func (m *Manager) ListSchemas(connID, database string) ([]string, error) {
	return m.impl.ListSchemas(connID, database)
}

func (m *Manager) ListTables(connID, database, schema string) ([]string, error) {
	return m.impl.ListTables(connID, database, schema)
}

func (m *Manager) GetColumns(connID, database, schema, table string) ([]QueryColumn, error) {
	cols, err := m.impl.GetColumns(connID, database, schema, table)
	if err != nil {
		return nil, err
	}
	out := make([]QueryColumn, len(cols))
	for i, c := range cols {
		out[i] = QueryColumn{Name: c.Name, DataType: c.DataType, Nullable: c.Nullable, Default: c.Default, Key: c.Key}
	}
	return out, nil
}

func (m *Manager) Query(connID, database, query string) (QueryResult, error) {
	r, err := m.impl.Query(connID, database, query)
	return QueryResult{Columns: r.Columns, Rows: r.Rows, RowCount: r.RowCount, Error: r.Error}, err
}

func LoadCreds() map[string]Cred {
	creds := dbexplorer.LoadCreds()
	out := make(map[string]Cred, len(creds))
	for k, c := range creds {
		out[k] = Cred{User: c.User, Password: c.Password}
	}
	return out
}

func SaveCreds(creds map[string]Cred) {
	out := make(map[string]dbexplorer.Cred, len(creds))
	for k, c := range creds {
		out[k] = dbexplorer.Cred{User: c.User, Password: c.Password}
	}
	dbexplorer.SaveCreds(out)
}

func RenderHTML(sessionID, dbType, portStr, remotePath, apiBase string) string {
	return dbexplorer.RenderHTML(sessionID, dbType, portStr, remotePath, apiBase)
}
